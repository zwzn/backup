package backend

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type FileBackend struct {
	root string
}

func init() {
	Register("file", func(u *url.URL) (Backend, error) {
		return NewFile(u.Host + u.Path), nil
	})
}

func NewFile(root string) Backend {
	return &FileBackend{
		root: root,
	}
}

func (b *FileBackend) URI() string {
	return "file://" + b.root
}

func (b *FileBackend) path(p string, t time.Time) string {
	return path.Join(b.root, fmt.Sprintf("%s-%d.gz", p, t.Unix()))
}

func (b *FileBackend) Write(p string, t time.Time, data io.Reader) error {
	newFile := b.path(p, t)
	err := os.MkdirAll(path.Dir(newFile), 0777)
	if err != nil {
		return err
	}

	f, err := os.Create(newFile)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	defer zw.Close()
	_, err = io.Copy(zw, data)
	if err != nil {
		return err
	}
	return nil
}

type FileFile struct {
	backend  *FileBackend
	name     string
	versions []time.Time
	isDir    bool
}

func (f *FileFile) Name() string {
	return f.name
}

func (f *FileFile) Versions() []time.Time {
	return f.versions
}

func (f *FileFile) IsDir() bool {
	return f.isDir
}

func (f *FileFile) Data(t time.Time) (io.ReadCloser, error) {
	p := f.backend.path(f.name, t)
	file, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(file)

	if err != nil {
		return nil, err
	}
	return zr, nil
}

func (b *FileBackend) List(p string) ([]File, error) {
	rawFiles, err := ioutil.ReadDir(path.Join(b.root, p))
	if err != nil {
		return nil, err
	}

	filesMap := map[string]*FileFile{}

	for _, rawFile := range rawFiles {
		if rawFile.IsDir() {
			filesMap[rawFile.Name()] = &FileFile{
				backend:  b,
				name:     rawFile.Name(),
				versions: []time.Time{},
				isDir:    true,
			}
		} else {
			filePath, t := splitName(rawFile.Name())
			file, ok := filesMap[filePath]
			if ok {
				file.versions = append(file.versions, t)
			} else {
				filesMap[filePath] = &FileFile{
					backend:  b,
					name:     filePath,
					versions: []time.Time{t},
					isDir:    false,
				}
			}
		}
	}

	files := []File{}
	for _, file := range filesMap {
		files = append(files, file)
	}
	return files, err
}

func (b *FileBackend) Read(p string) (File, error) {
	versions := []time.Time{}

	dir, name := path.Split(p)

	rawFiles, err := ioutil.ReadDir(path.Join(b.root, dir))
	if err != nil {
		return nil, err
	}

	for _, f := range rawFiles {
		if !f.IsDir() {
			n, t := splitName(f.Name())
			if n == name {
				versions = append(versions, t)
			}
		}
	}

	if len(versions) == 0 {
		return nil, os.ErrNotExist
	}

	return &FileFile{
		backend:  b,
		name:     p,
		versions: versions,
		isDir:    true,
	}, nil
}

func splitName(name string) (string, time.Time) {
	parts := strings.Split(name, "-")

	unixStr := parts[len(parts)-1]
	unixStr = unixStr[:len(unixStr)-3]
	unix, _ := strconv.Atoi(unixStr)
	t := time.Unix(int64(unix), 0)
	return strings.Join(parts[:len(parts)-1], "-"), t
}
