package backend

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
)

type File struct {
	root string
}

func init() {
	Register("file", func(u *url.URL) (Backend, error) {
		return NewFile(u.Host + u.Path), nil
	})
}

func NewFile(root string) Backend {
	return &File{
		root: root,
	}
}

func (b *File) URI() string {
	return "file://" + b.root
}

func (b *File) Push(p string, time int64, data io.Reader) error {
	newFile := path.Join(b.root, fmt.Sprintf("%s-%d.gz", p, time))
	err := os.MkdirAll(path.Dir(newFile), 0777)
	if err != nil {
		return err
	}

	f, err := os.Create(newFile)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(gzip.NewWriter(f), data)
	if err != nil {
		return err
	}
	return nil
}
