package backend

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPFile struct {
	backend  *SFTPBackend
	name     string
	versions []time.Time
	isDir    bool
}

func (f *SFTPFile) Name() string {
	return f.name
}

func (f *SFTPFile) Versions() []time.Time {
	return f.versions
}

func (f *SFTPFile) IsDir() bool {
	return f.isDir
}

func (f *SFTPFile) Data(t time.Time) (io.ReadCloser, error) {
	p := f.backend.path(f.name, t)
	file, err := f.backend.sftpClient.Open(p)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(file)

	if err != nil {
		return nil, err
	}
	return zr, nil
}

type SFTPBackend struct {
	root       string
	sftpClient *sftp.Client
	sshClient  *ssh.Client
}

func init() {
	Register("sftp", func(u *url.URL) (Backend, error) {
		user := u.User.Username()
		pass, _ := u.User.Password()
		remote := u.Hostname()
		port := ":" + u.Port()

		// get host public key
		// hostKey := getHostKey(remote)

		config := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(pass),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			// HostKeyCallback: ssh.FixedHostKey(hostKey),
		}

		// connect
		conn, err := ssh.Dial("tcp", remote+port, config)
		if err != nil {
			return nil, err
		}

		// create new SFTP client
		client, err := sftp.NewClient(conn)
		if err != nil {
			return nil, err
		}

		return NewSFTP(client, conn, u.Path), nil
	})
}

func NewSFTP(sftpClient *sftp.Client, sshClient *ssh.Client, root string) Backend {
	return &SFTPBackend{
		sftpClient: sftpClient,
		sshClient:  sshClient,
		root:       root,
	}
}

func (b *SFTPBackend) URI() string {
	return "sftp://" + sftp.Join(b.sshClient.RemoteAddr().String(), b.root)
}

func (b *SFTPBackend) path(p string, t time.Time) string {
	return path.Join(b.root, fmt.Sprintf("%s-%d.gz", p, t.Unix()))
}

func (b *SFTPBackend) Write(p string, t time.Time, data io.Reader) error {
	newFile := b.path(p, t)
	err := b.sftpClient.MkdirAll(path.Dir(newFile))
	if err != nil {
		return err
	}

	f, err := b.sftpClient.Create(newFile)
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

func (b *SFTPBackend) List(p string) ([]File, error) {
	rawFiles, err := b.sftpClient.ReadDir(path.Join(b.root, p))
	if err != nil {
		return nil, err
	}

	filesMap := map[string]*SFTPFile{}

	for _, rawFile := range rawFiles {
		if rawFile.IsDir() {
			filesMap[rawFile.Name()] = &SFTPFile{
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
				filesMap[filePath] = &SFTPFile{
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

func (b *SFTPBackend) Read(p string) (File, error) {
	versions := []time.Time{}

	dir, name := path.Split(p)

	rawFiles, err := b.sftpClient.ReadDir(path.Join(b.root, dir))
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

	return &SFTPFile{
		backend:  b,
		name:     p,
		versions: versions,
		isDir:    true,
	}, nil
}

func (b *SFTPBackend) Close() error {
	sftpErr := b.sftpClient.Close()
	sshErr := b.sshClient.Close()
	if sftpErr != nil {
		return sftpErr
	}
	if sshErr != nil {
		return sshErr
	}
	return nil
}

func getHostKey(host string) ssh.PublicKey {
	// parse OpenSSH known_hosts file
	// ssh or use ssh-keyscan to get initial key
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], host) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				log.Fatalf("error parsing %q: %v", fields[2], err)
			}
			break
		}
	}

	if hostKey == nil {
		log.Fatalf("no hostkey found for %s", host)
	}

	return hostKey
}
