package backend

import (
	"fmt"
	"io"
	"net/url"
)

var ErrNoBackend = fmt.Errorf("no supported backend")

type Backend interface {
	URI() string
	Push(path string, date int64, data io.Reader) error
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
