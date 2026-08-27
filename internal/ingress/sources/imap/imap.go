// Package imap implements an IngestorSource backed by an IMAP mailbox.
// Each Poll() and Fetch() call opens its own connection and closes it before returning.
package imap

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"

	"damask/server/internal/ingress"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func init() {
	ingress.Register("imap", New)
}

// Config is the decrypted JSON configuration for an IMAP source.
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TLS      bool   `json:"tls"`
	Username string `json:"username"`
	Password string `json:"password"`
	Mailbox  string `json:"mailbox"`
}

// Source pulls messages from an IMAP mailbox.
type Source struct {
	cfg Config
}

// New builds an IMAPSource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("imap: parse config: %w", err)
	}
	if cfg.Mailbox == "" {
		cfg.Mailbox = "INBOX"
	}
	if cfg.Port == 0 {
		if cfg.TLS {
			cfg.Port = 993
		} else {
			cfg.Port = 143
		}
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Type() string { return "imap" }

func (s *Source) Validate(_ context.Context) error {
	c, err := s.connect()
	if err != nil {
		return err
	}
	defer c.Close()
	cmd := c.Select(s.cfg.Mailbox, nil)
	if _, waitErr := cmd.Wait(); waitErr != nil {
		return fmt.Errorf("imap: select %s: %w", s.cfg.Mailbox, waitErr)
	}
	return nil
}

func (s *Source) Poll(_ context.Context) ([]ingress.IngestItem, error) {
	c, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	if _, waitErr := c.Select(s.cfg.Mailbox, nil).Wait(); waitErr != nil {
		return nil, fmt.Errorf("imap: select %s: %w", s.cfg.Mailbox, waitErr)
	}

	// Search for all UNSEEN messages
	criteria := &goimap.SearchCriteria{
		NotFlag: []goimap.Flag{goimap.FlagSeen},
	}
	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("imap: search: %w", err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	// Fetch envelopes for all matched UIDs
	numSet := goimap.UIDSetNum(uids...)
	fetchOpts := &goimap.FetchOptions{
		Envelope: true,
		UID:      true,
	}
	fetchCmd := c.Fetch(numSet, fetchOpts)
	defer fetchCmd.Close()

	var items []ingress.IngestItem
	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}
		buf, collectErr := msg.Collect()
		if collectErr != nil {
			continue
		}
		if buf.Envelope == nil {
			continue
		}
		uid := buf.UID
		subject := ""
		if buf.Envelope != nil {
			subject = buf.Envelope.Subject
		}
		// One item per message; filename is subject or UID-based fallback
		filename := subject
		if filename == "" {
			filename = "message-" + strconv.FormatUint(uint64(uid), 10) + ".eml"
		}
		items = append(items, ingress.IngestItem{
			RemoteID: strconv.FormatUint(uint64(uid), 10),
			Filename: filename,
			Meta:     map[string]string{"subject": subject},
		})
	}

	return items, nil
}

// Fetch downloads the raw RFC 5322 message for the given item.
// It opens a fresh connection and wraps it so Close() closes the connection.
func (s *Source) Fetch(_ context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	uidVal, err := strconv.ParseUint(item.RemoteID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("imap: parse uid %s: %w", item.RemoteID, err)
	}
	uid := goimap.UID(uidVal)

	c, err := s.connect()
	if err != nil {
		return nil, err
	}

	if _, waitErr := c.Select(s.cfg.Mailbox, nil).Wait(); waitErr != nil {
		_ = c.Close()
		return nil, fmt.Errorf("imap: select %s: %w", s.cfg.Mailbox, waitErr)
	}

	numSet := goimap.UIDSetNum(uid)
	fetchOpts := &goimap.FetchOptions{
		BodySection: []*goimap.FetchItemBodySection{{}}, // BODY[] — entire message
	}
	fetchCmd := c.Fetch(numSet, fetchOpts)
	msg := fetchCmd.Next()
	if msg == nil {
		_ = fetchCmd.Close()
		_ = c.Close()
		return nil, fmt.Errorf("imap: message uid %d not found", uid)
	}

	for {
		item := msg.Next()
		if item == nil {
			break
		}
		if bs, ok := item.(imapclient.FetchItemDataBodySection); ok {
			// Wrap reader to close the IMAP connection when done
			return &imapReadCloser{r: bs.Literal, cmd: fetchCmd, conn: c}, nil
		}
	}

	_ = fetchCmd.Close()
	_ = c.Close()
	return nil, fmt.Errorf("imap: no body section in fetch response for uid %d", uid)
}

// connect opens an authenticated IMAP connection.
func (s *Source) connect() (*imapclient.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	var (
		c   *imapclient.Client
		err error
	)
	if s.cfg.TLS {
		c, err = imapclient.DialTLS(addr, &imapclient.Options{
			TLSConfig: &tls.Config{ServerName: s.cfg.Host},
		})
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("imap: connect %s: %w", addr, err)
	}
	if loginErr := c.Login(s.cfg.Username, s.cfg.Password).Wait(); loginErr != nil {
		_ = c.Close()
		return nil, fmt.Errorf("imap: login: %w", loginErr)
	}
	return c, nil
}

// imapReadCloser wraps an IMAP body literal reader.
// Close() drains and closes the fetch command then the connection.
type imapReadCloser struct {
	r    io.Reader
	cmd  *imapclient.FetchCommand
	conn *imapclient.Client
}

func (rc *imapReadCloser) Read(p []byte) (int, error) { return rc.r.Read(p) }
func (rc *imapReadCloser) Close() error {
	_ = rc.cmd.Close()
	return rc.conn.Close()
}
