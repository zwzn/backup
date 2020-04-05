package backend

import (
	"fmt"
	"io"
	"os"
	"path"
)

type File struct {
	root string
}

func NewFile(root string) Backend {
	return &File{
		root: root,
	}
}

func (b *File) push(p string, time int64, data io.Reader) error {
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
	_, err = io.Copy(f, data)
	if err != nil {
		return err
	}
	return nil
}
