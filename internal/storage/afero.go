// Package storage abstracts file storage backends.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3 "github.com/fclairamb/afero-s3"
	"github.com/spf13/afero"
)

type AferoStorage struct {
	base string
	fs   afero.Fs
}

func NewAferoMemoryStorage() (Storage, error) {
	base := "/"
	fs := afero.NewMemMapFs()
	return &AferoStorage{base, fs}, nil
}

type AferoS3Config struct {
	Base      string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

func NewAferoS3Storage(cfg AferoS3Config) (Storage, error) {
	fs := s3.NewFsFromConfig(cfg.Bucket, aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	})
	base := strings.Trim(cfg.Base, "/")
	return &AferoStorage{base, fs}, nil
}

func (s *AferoStorage) Put(ctx context.Context, key string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dst := filepath.Join(s.base, filepath.FromSlash(key))
	if err := s.fs.MkdirAll(filepath.Dir(dst), 0755); err != nil { //nolint:mnd // 0755 is appropriate for directories
		return err
	}
	f, err := s.fs.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *AferoStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := s.fs.Open(filepath.Join(s.base, filepath.FromSlash(key)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("afero storage: get %s: %w", key, ErrNotFound)
		}
		return nil, err
	}
	return f, nil
}

func (s *AferoStorage) Stat(ctx context.Context, key string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	fi, err := s.fs.Stat(filepath.Join(s.base, filepath.FromSlash(key)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Info{}, fmt.Errorf("afero storage: stat %s: %w", key, ErrNotFound)
		}
		return Info{}, err
	}
	return Info{Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

func (s *AferoStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.fs.RemoveAll(filepath.Join(s.base, filepath.FromSlash(key)))
}

func (s *AferoStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := filepath.Join(s.base, filepath.FromSlash(prefix))

	exists, err := afero.DirExists(s.fs, root)
	if err != nil || !exists {
		return []string{}, nil //nolint:nilerr // treat non-existent prefix as empty list
	}

	var keys []string
	err = afero.Walk(s.fs, root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(s.base, path)
			keys = append(keys, strings.ReplaceAll(rel, string(filepath.Separator), "/"))
		}
		return nil
	})

	return keys, err
}
