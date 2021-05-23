package backup

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/abibby/backup/backend"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type Options struct {
	Backends []backend.Backend
	Ignore   []string
}

func printTime(start time.Time) {
	t := time.Since(start)
	if t > time.Second {
		t = t.Truncate(time.Second)
	}
	if t > time.Millisecond {
		t = t.Truncate(time.Millisecond)
	}
	fmt.Println(t)
}

func Backup(dir string, o *Options) error {
	db, err := bbolt.Open("./db.bolt", 0644, nil)
	if err != nil {
		return errors.Wrap(err, "failed to initialize database")
	}
	for _, b := range o.Backends {
		err = db.Update(func(tx *bbolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(b.URI()))
			return err
		})
		if err != nil {
			return errors.Wrap(err, "failed to create database bucket")
		}
	}
	defer printTime(time.Now())
	err = backupFolder(dir, db, o)
	return err
}

func backupFolder(dir string, db *bbolt.DB, o *Options) error {
	var err error
	files, err := ioutil.ReadDir(dir)

	if err != nil {
		return errors.Wrapf(err, "failed to load directory %s", dir)
	}

	for _, f := range files {
		p := path.Join(dir, f.Name())
		if matches(p, o.Ignore) {
			continue
		}
		if f.IsDir() {
			err = backupFolder(p, db, o)
			if err != nil {
				return err
			}
		} else if f.Mode()&os.ModeSymlink != 0 {
		} else {
			for _, b := range o.Backends {
				err = backupFile(db, b, p, f)
				if err != nil {
					log.Printf("failed to backup file %s: %v\n", p, err)
				}
			}

		}
	}
	return nil
}

func backupFile(db *bbolt.DB, b backend.Backend, p string, f os.FileInfo) error {
	ut, err := getUpdatedTime(b.URI(), p, db)
	if err != nil {
		return err
	}

	if ut < f.ModTime().Unix() {
		t := time.Now()

		file, err := os.Open(p)
		if err != nil {
			return err
		}

		err = b.Write(p, t, file)
		if err != nil {
			return err
		}
		file.Close()

		err = setUpdatedTime(b.URI(), p, db, t)
		if err != nil {
			return err
		}
	}
	return nil
}

func getUpdatedTime(config, path string, db *bbolt.DB) (int64, error) {
	var t int64
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(config))
		by := b.Get([]byte(path))

		if by != nil {
			t = int64(binary.LittleEndian.Uint64(by))
		}

		return nil
	})

	return t, errors.Wrap(err, "failed to read database")
}

func setUpdatedTime(config, path string, db *bbolt.DB, t time.Time) error {
	err := db.Update(func(tx *bbolt.Tx) error {
		timeBytes := make([]byte, 8)
		b := tx.Bucket([]byte(config))
		binary.LittleEndian.PutUint64(timeBytes, uint64(t.Unix()))
		return b.Put([]byte(path), timeBytes)
	})

	return errors.Wrap(err, "failed to update database")
}

var regexCache = map[string]*regexp.Regexp{}

func toRegex(glob string) *regexp.Regexp {
	re, ok := regexCache[glob]
	if !ok {
		strRe := ""
		if strings.HasPrefix(glob, "/") {
			glob = glob[1:]
			strRe += "^"
		}
		reParts := []string{}
		for _, part := range strings.Split(glob, "/") {
			part = strings.ReplaceAll(part, "**", "👏")
			part = strings.ReplaceAll(part, "*", `[^\/]*`)
			part = strings.ReplaceAll(part, "👏", `.*`)
			reParts = append(reParts, part)
		}
		strRe += strings.Join(reParts, `\/`)
		strRe += "$"

		re = regexp.MustCompile(strRe)
		regexCache[glob] = re
	}

	return re
}

func matches(s string, globs []string) bool {
	for _, glob := range globs {
		if toRegex(glob).MatchString(s) {
			return true
		}
	}
	return false
}
