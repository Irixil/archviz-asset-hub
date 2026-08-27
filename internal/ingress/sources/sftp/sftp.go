// Package sftp implements an IngestorSource backed by an SFTP server.
package sftp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"damask/server/internal/ingress"

	gsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func init() {
	ingress.Register("sftp", New)
}

// Config is the decrypted JSON configuration for an SFTP source.
type Config struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	AuthMethod         string `json:"auth_method"` // "password" | "key"
	Password           string `json:"password"`
	PrivateKey         string `json:"private_key"` // PEM-encoded
	RemotePath         string `json:"remote_path"`
	Recursive          bool   `json:"recursive"`
	HostKeyFingerprint string `json:"host_key_fingerprint"`
	InsecureHostKey    bool   `json:"insecure_host_key"` // skip host-key verification; not recommended for production
}

// Source watches a remote directory for new files.
type Source struct {
	cfg Config
}

// New builds an SFTPSource from decrypted config JSON.
func New(configJSON []byte) (ingress.Source, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("sftp: parse config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.RemotePath == "" {
		cfg.RemotePath = "/"
	}
	return &Source{cfg: cfg}, nil
}

func (s *Source) Type() string { return "sftp" }

func (s *Source) Validate(_ context.Context) error {
	client, sshConn, err := s.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	fi, err := client.Stat(s.cfg.RemotePath)
	if err != nil {
		return fmt.Errorf("sftp: stat %s: %w", s.cfg.RemotePath, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("sftp: %s is not a directory", s.cfg.RemotePath)
	}
	return nil
}

func (s *Source) Poll(ctx context.Context) ([]ingress.IngestItem, error) {
	client, sshConn, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer sshConn.Close()

	var items []ingress.IngestItem
	if err := s.pollDirectory(ctx, client, s.cfg.RemotePath, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Source) pollDirectory(ctx context.Context, client *gsftp.Client, dir string, items *[]ingress.IngestItem) error {
	entries, err := client.ReadDirContext(ctx, dir)
	if err != nil {
		return fmt.Errorf("sftp: readdir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		remotePath := path.Join(dir, entry.Name())
		if entry.IsDir() {
			if s.cfg.Recursive && !strings.HasPrefix(entry.Name(), ".") {
				if err := s.pollDirectory(ctx, client, remotePath, items); err != nil {
					return err
				}
			}
			continue
		}
		if shouldSkipFile(entry.Name()) {
			continue
		}
		*items = append(*items, ingress.IngestItem{
			RemoteID: remotePath,
			Filename: entry.Name(),
			ModTime:  entry.ModTime(),
			Size:     entry.Size(),
		})
	}
	return nil
}

func shouldSkipFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(name, ".") ||
		strings.HasSuffix(lower, ".tmp") ||
		strings.HasSuffix(lower, ".part") ||
		strings.HasSuffix(lower, ".crdownload")
}

func (s *Source) Fetch(_ context.Context, item ingress.IngestItem) (io.ReadCloser, error) {
	client, sshConn, err := s.connect()
	if err != nil {
		return nil, err
	}

	f, err := client.Open(item.RemoteID)
	if err != nil {
		_ = client.Close()
		_ = sshConn.Close()
		return nil, fmt.Errorf("sftp: open %s: %w", item.RemoteID, err)
	}

	return &sftpReadCloser{f: f, client: client, ssh: sshConn}, nil
}

func (s *Source) connect() (*gsftp.Client, *ssh.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))

	var authMethods []ssh.AuthMethod
	if s.cfg.AuthMethod == "key" && s.cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.cfg.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("sftp: parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(s.cfg.Password))
	}

	hostKeyCallback := ssh.HostKeyCallback(func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if s.cfg.HostKeyFingerprint != "" {
			actual := ssh.FingerprintSHA256(key)
			if actual == s.cfg.HostKeyFingerprint {
				return nil
			}
			return fmt.Errorf("sftp: host key fingerprint mismatch: got %s", actual)
		}
		return errors.New(
			"sftp: host key verification not configured; " +
				"set host_key_fingerprint or explicitly allow insecure_host_key")
	})
	if s.cfg.InsecureHostKey && s.cfg.HostKeyFingerprint == "" {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // config
	}
	sshCfg := &ssh.ClientConfig{
		User:            s.cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	sshConn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("sftp: ssh dial %s: %w", addr, err)
	}

	client, err := gsftp.NewClient(sshConn)
	if err != nil {
		_ = sshConn.Close()
		return nil, nil, fmt.Errorf("sftp: new client: %w", err)
	}
	return client, sshConn, nil
}

type sftpReadCloser struct {
	f      *gsftp.File
	client *gsftp.Client
	ssh    *ssh.Client
}

func (rc *sftpReadCloser) Read(p []byte) (int, error) { return rc.f.Read(p) }
func (rc *sftpReadCloser) Close() error {
	_ = rc.f.Close()
	_ = rc.client.Close()
	return rc.ssh.Close()
}
