package filestore

import (
	"io"
	"os"
	"path/filepath"
)

type Local struct {
	Dir string
}

func (l Local) SaveQR(userID string, ext string, src io.Reader) (string, error) {
	if err := os.MkdirAll(l.Dir, 0755); err != nil {
		return "", err
	}
	name := userID + ext
	path := filepath.Join(l.Dir, name)
	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

func (l Local) DeleteURL(url string) error {
	if url == "" || l.Dir == "" {
		return nil
	}
	return os.Remove(filepath.Join(l.Dir, filepath.Base(url)))
}
