package backend

import (
	"fmt"
	"io"
	"net/url"
	"time"
)

var ErrNoBackend = fmt.Errorf("no supported backend")

type File interface {
	Name() string
	Versions() []time.Time
	IsDir() bool
	Data(time.Time) (io.ReadCloser, error)
}

type Backend interface {
	URI() string
	Write(path string, date time.Time, data io.Reader) error
	List(path string) ([]File, error)
	Read(path string) (File, error)
}

type Closer interface {
	Close() error
}

func Load(connection string) (Backend, error) {
	u, err := url.Parse(connection)
	if err != nil {
		return nil, err
	}

	creator, ok := backends[u.Scheme]
	if !ok {
		return nil, ErrNoBackend
	}
	return creator(u)
}

var backends = map[string]func(u *url.URL) (Backend, error){}

func Register(scheme string, creator func(u *url.URL) (Backend, error)) {
	backends[scheme] = creator
}
