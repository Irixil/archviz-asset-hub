package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	gsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPConfig holds the connection parameters for SFTPStorage.
type SFTPConfig struct {
	Host            string
	Port            int // default 22
	User            string
	AuthMethod      string // "password" | "key"
	Password        string
	PrivateKey      string // PEM-encoded
	BasePath        string // remote base directory, default "/"
	InsecureHostKey bool   // skip host-key verification; set via STORAGE_SFTP_INSECURE_HOST_KEY=true
}

// SFTPStorage implements Storage backed by a remote SFTP server.
// Each operation opens a fresh connection; connections are not pooled.
type SFTPStorage struct {
	cfg SFTPConfig
}

// NewSFTPStorage validates the config and returns an SFTPStorage.
// It does not dial at construction time so a temporarily unavailable
// SFTP server does not prevent the application from starting.
func NewSFTPStorage(cfg SFTPConfig) (Storage, error) {
	if cfg.Host == "" {
		return nil, errors.New("sftp storage: STORAGE_SFTP_HOST is required")
	}
	if cfg.User == "" {
		return nil, errors.New("sftp storage: STORAGE_SFTP_USER is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.BasePath == "" {
		cfg.BasePath = "/"
	}
	return &SFTPStorage{cfg: cfg}, nil
}

func (s *SFTPStorage) connect() (*gsftp.Client, *ssh.Client, error) {
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))

	var authMethods []ssh.AuthMethod
	if s.cfg.AuthMethod == "key" && s.cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.cfg.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("sftp storage: parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(s.cfg.Password))
	}

	hostKeyCallback := ssh.HostKeyCallback(func(_ string, _ net.Addr, _ ssh.PublicKey) error {
		return errors.New(
			"sftp storage: host key verification not configured;" +
				"set STORAGE_SFTP_INSECURE_HOST_KEY=true to skip (not recommended for production)")
	})
	if s.cfg.InsecureHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // config
	}
	sshCfg := &ssh.ClientConfig{
		User:            s.cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}
	sshConn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("sftp storage: ssh dial %s: %w", addr, err)
	}

	client, err := gsftp.NewClient(sshConn)
	if err != nil {
		_ = sshConn.Close()
		return nil, nil, fmt.Errorf("sftp storage: new client: %w", err)
	}
	return client, sshConn, nil
}

func (s *SFTPStorage) remotePath(key string) string {
	return path.Join(s.cfg.BasePath, key)
}

func (s *SFTPStorage) Put(ctx context.Context, key string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, sshConn, err := s.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	remote := s.remotePath(key)
	if err = client.MkdirAll(path.Dir(remote)); err != nil {
		return fmt.Errorf("sftp storage: mkdirall %s: %w", path.Dir(remote), err)
	}

	f, err := client.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sftp storage: open %s: %w", remote, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, r); err != nil {
		return fmt.Errorf("sftp storage: write %s: %w", remote, err)
	}
	return nil
}

func (s *SFTPStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client, sshConn, err := s.connect()
	if err != nil {
		return nil, err
	}

	remote := s.remotePath(key)
	f, err := client.Open(remote)
	if err != nil {
		_ = client.Close()
		_ = sshConn.Close()
		if isNotExist(err) {
			return nil, fmt.Errorf("sftp storage: open %s: %w", remote, ErrNotFound)
		}
		return nil, fmt.Errorf("sftp storage: open %s: %w", remote, err)
	}
	return &sftpReadCloser{f: f, client: client, ssh: sshConn}, nil
}

func (s *SFTPStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, sshConn, err := s.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	remote := s.remotePath(key)
	if err = client.Remove(remote); err != nil && !isNotExist(err) {
		return fmt.Errorf("sftp storage: remove %s: %w", remote, err)
	}

	// Best-effort: remove empty parent directories up to (but not including) basePath.
	dir := path.Dir(remote)
	for dir != s.cfg.BasePath && dir != "/" && dir != "." {
		if err = client.RemoveDirectory(dir); err != nil {
			break // non-empty or permission error — stop climbing
		}
		dir = path.Dir(dir)
	}
	return nil
}

func (s *SFTPStorage) Stat(ctx context.Context, key string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	client, sshConn, err := s.connect()
	if err != nil {
		return Info{}, err
	}
	defer client.Close()
	defer sshConn.Close()

	remote := s.remotePath(key)
	fi, err := client.Stat(remote)
	if err != nil {
		if isNotExist(err) {
			return Info{}, fmt.Errorf("sftp storage: stat %s: %w", remote, ErrNotFound)
		}
		return Info{}, fmt.Errorf("sftp storage: stat %s: %w", remote, err)
	}
	return Info{Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

func (s *SFTPStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	client, sshConn, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer sshConn.Close()

	root := path.Join(s.cfg.BasePath, prefix)

	if _, err = client.Stat(root); err != nil {
		if isNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("sftp storage: stat %s: %w", root, err)
	}

	walker := client.Walk(root)
	var keys []string
	for walker.Step() {
		if e := walker.Err(); e != nil {
			return nil, fmt.Errorf("sftp storage: walk %s: %w", root, e)
		}
		if walker.Stat().IsDir() {
			continue
		}
		// Strip leading basePath + "/" to produce a relative key.
		rel := strings.TrimPrefix(walker.Path(), s.cfg.BasePath+"/")
		keys = append(keys, rel)
	}
	return keys, nil
}

// isNotExist reports whether the error indicates a missing file on the remote server.
func isNotExist(err error) bool {
	statusErr := &gsftp.StatusError{}
	if errors.As(err, &statusErr) {
		return statusErr.FxCode() == gsftp.ErrSSHFxNoSuchFile
	}
	return false
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
