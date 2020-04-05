package backend

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

type Backend interface {
	push(path string, date int64, data io.Reader) error
}

func Push(backend Backend, path string, date int64, data io.Reader) error {
	fmt.Printf("save %s\n", path)
	rBuf := bytes.NewBuffer([]byte(""))
	_, err := io.Copy(gzip.NewWriter(rBuf), data)
	if err != nil {
		return err
	}
	return backend.push(path, date, rBuf)
}
