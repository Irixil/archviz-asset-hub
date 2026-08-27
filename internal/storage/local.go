package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	base string
}

func NewLocalStorage(base string) (Storage, error) {
	if err := os.MkdirAll(base, 0750); err != nil {
		return nil, err
	}
	return &LocalStorage{base: base}, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, r io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dst := filepath.Join(s.base, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(s.base, filepath.FromSlash(key)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("local storage: get %s: %w", key, ErrNotFound)
		}
		return nil, err
	}
	return f, nil
}

func (s *LocalStorage) Stat(ctx context.Context, key string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	fi, err := os.Stat(filepath.Join(s.base, filepath.FromSlash(key)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Info{}, fmt.Errorf("local storage: stat %s: %w", key, ErrNotFound)
		}
		return Info{}, err
	}
	return Info{Size: fi.Size(), ModTime: fi.ModTime()}, nil
}

func (s *LocalStorage) LocalPath(key string) string {
	return filepath.Join(s.base, filepath.FromSlash(key))
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.base, filepath.FromSlash(key)))
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := filepath.Join(s.base, filepath.FromSlash(prefix))
	var keys []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(s.base, path)
			keys = append(keys, strings.ReplaceAll(rel, string(filepath.Separator), "/"))
		}
		return nil
	})
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	return keys, err
}
