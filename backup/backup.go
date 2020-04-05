package backup

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/zwzn/backup/backend"
	"go.etcd.io/bbolt"
)

type Options struct {
	Backend backend.Backend
	Ignore  []string
}

func printTime(start time.Time) {
	fmt.Println(time.Since(start).Truncate(time.Second))
}

func Backup(dir string, o *Options) error {
	db, err := bbolt.Open("./db.bolt", 0644, nil)
	if err != nil {
		return errors.Wrap(err, "failed to initialize database")
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("files"))
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to create database bucket")
	}

	defer printTime(time.Now())
	err = backup(dir, db, o)
	return err
}

func backup(dir string, db *bbolt.DB, o *Options) error {
	var err error
	files, err := ioutil.ReadDir(dir)

	if err != nil {
		return errors.Wrapf(err, "failed to load directory %s", dir)
	}

	for _, f := range files {
		p := path.Join(dir, f.Name())
		if containsAny(p, o.Ignore) {
			continue
		}
		if f.IsDir() {
			err = backup(p, db, o)
			if err != nil {
				return err
			}
		} else {
			ut, err := getUpdatedTime(p, db)
			if err != nil {
				return err
			}

			if ut < f.ModTime().Unix() {
				t := time.Now().Unix()

				file, err := os.Open(p)
				if err != nil {
					panic(err)
				}
				err = backend.Push(o.Backend, p, t, file)
				if err != nil {
					panic(err)
				}
				file.Close()

				err = setUpdatedTime(p, db, t)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func getUpdatedTime(path string, db *bbolt.DB) (int64, error) {
	var t int64
	err := db.View(func(tx *bbolt.Tx) error {
		by := tx.
			Bucket([]byte("files")).
			Get([]byte(path))

		if by != nil {
			t = int64(binary.LittleEndian.Uint64(by))
		}

		return nil
	})

	return t, errors.Wrap(err, "failed to read database")
}

func setUpdatedTime(path string, db *bbolt.DB, t int64) error {
	err := db.Update(func(tx *bbolt.Tx) error {
		timeBytes := make([]byte, 8)
		b := tx.Bucket([]byte("files"))
		binary.LittleEndian.PutUint64(timeBytes, uint64(t))
		return b.Put([]byte(path), timeBytes)
	})

	return errors.Wrap(err, "failed to update database")
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
