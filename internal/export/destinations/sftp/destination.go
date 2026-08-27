// Package sftp implements an ExportDestination backed by an SFTP server.
package sftp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"strconv"

	"damask/server/internal/export"

	gsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func init() {
	export.Register("sftp", func(cfg []byte) (export.Destination, error) {
		return New(cfg)
	})
}

// Config is the decrypted JSON configuration for an SFTP export destination.
type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	PrivateKey      string `json:"private_key"` // PEM-encoded
	RemotePath      string `json:"remote_path"`
	InsecureHostKey bool   `json:"insecure_host_key"` // skip host-key verification; not recommended for production
}

// Destination writes exports to an SFTP server.
type Destination struct {
	cfg Config
}

// New builds an SFTP Destination from decrypted config JSON.
func New(configJSON []byte) (*Destination, error) {
	var cfg Config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("sftp dest: parse config: %w", err)
	}
	if cfg.Host == "" {
		return nil, errors.New("sftp dest: host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Password == "" && cfg.PrivateKey == "" {
		return nil, errors.New("sftp dest: password or private_key is required")
	}
	return &Destination{cfg: cfg}, nil
}

func (d *Destination) Type() string { return "sftp" }

func (d *Destination) Write(_ context.Context, remotePath string, r io.Reader, _ int64, _ string) error {
	client, sshConn, err := d.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	dir := path.Dir(path.Join(d.cfg.RemotePath, remotePath))
	if err = client.MkdirAll(dir); err != nil {
		return fmt.Errorf("sftp dest: mkdir %s: %w", dir, err)
	}

	fullPath := path.Join(d.cfg.RemotePath, remotePath)
	f, err := client.Create(fullPath)
	if err != nil {
		return fmt.Errorf("sftp dest: create %s: %w", fullPath, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, r); err != nil {
		return fmt.Errorf("sftp dest: write %s: %w", fullPath, err)
	}
	return nil
}

func (d *Destination) ReadManifest(_ context.Context, remotePath string) ([]byte, error) {
	client, sshConn, err := d.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer sshConn.Close()

	fullPath := path.Join(d.cfg.RemotePath, remotePath)
	f, err := client.Open(fullPath)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sftp dest: open manifest %s: %w", fullPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("sftp dest: read manifest: %w", err)
	}
	return data, nil
}

func (d *Destination) WriteManifest(_ context.Context, remotePath string, data []byte) error {
	client, sshConn, err := d.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	fullPath := path.Join(d.cfg.RemotePath, remotePath)
	tmpPath := fullPath + ".tmp"

	f, err := client.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("sftp dest: create tmp manifest: %w", err)
	}
	if _, err = io.Copy(f, bytes.NewReader(data)); err != nil {
		_ = f.Close()
		return fmt.Errorf("sftp dest: write tmp manifest: %w", err)
	}
	_ = f.Close()

	if err = client.Rename(tmpPath, fullPath); err != nil {
		return fmt.Errorf("sftp dest: rename manifest: %w", err)
	}
	return nil
}

func (d *Destination) Validate(_ context.Context) error {
	client, sshConn, err := d.connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer sshConn.Close()

	fi, err := client.Stat(d.cfg.RemotePath)
	if err != nil {
		return fmt.Errorf("sftp dest: stat %s: %w", d.cfg.RemotePath, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("sftp dest: %s is not a directory", d.cfg.RemotePath)
	}

	probePath := path.Join(d.cfg.RemotePath, ".damask_probe")
	pf, err := client.Create(probePath)
	if err != nil {
		return fmt.Errorf("sftp dest: create probe file: %w", err)
	}
	_, _ = pf.Write([]byte{1})
	_ = pf.Close()
	_ = client.Remove(probePath)

	return nil
}

func (d *Destination) connect() (*gsftp.Client, *ssh.Client, error) {
	addr := net.JoinHostPort(d.cfg.Host, strconv.Itoa(d.cfg.Port))

	var authMethods []ssh.AuthMethod
	if d.cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(d.cfg.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("sftp dest: parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(d.cfg.Password))
	}

	hostKeyCallback := ssh.HostKeyCallback(func(_ string, _ net.Addr, _ ssh.PublicKey) error {
		return errors.New("sftp dest: host key verification not configured; " +
			"set insecure_host_key=true in dest config to skip (not recommended for production)")
	})
	if d.cfg.InsecureHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // config opt-in
	}
	sshCfg := &ssh.ClientConfig{
		User:            d.cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}
	sshConn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("sftp dest: ssh dial %s: %w", addr, err)
	}
	client, err := gsftp.NewClient(sshConn)
	if err != nil {
		_ = sshConn.Close()
		return nil, nil, fmt.Errorf("sftp dest: new client: %w", err)
	}
	return client, sshConn, nil
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, gsftp.ErrSSHFxNoSuchFile)
}
