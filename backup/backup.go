package backup

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type Options struct {
	db     *bbolt.DB
	ignore []string
}

func printTime(start time.Time) {
	fmt.Println(time.Since(start).Truncate(time.Second))
}

func Backup(dir string) error {
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
	err = backup(dir, &Options{
		db: db,
		ignore: []string{
			"node_modules",
			"/home/adam/.cache/*",
		},
	})
	return err
}

func backup(dir string, o *Options) error {
	// spew.Dump(dir)
	var err error
	files, err := ioutil.ReadDir(dir)
	currentTime := int64(0)
	needsUpdate := false

	if err != nil {
		return errors.Wrapf(err, "failed to load directory %s", dir)
	}

	for _, f := range files {
		p := path.Join(dir, f.Name())
		for _, g := range o.ignore {
			if strings.Contains(p, g) {
				continue
			}
		}
		if f.IsDir() {
			err = backup(p, o)
			if err != nil {
				return err
			}
		} else {
			currentTime = time.Now().Unix()
			ut, err := getUpdatedTime(p, o.db)
			if err != nil {
				return err
			}

			if ut < f.ModTime().Unix() {
				err = setUpdatedTime(p, o.db)
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

		t = int64(binary.LittleEndian.Uint64(by))

		return nil
	})

	return t, errors.Wrap(err, "failed to read database")
}

func setUpdatedTime(path string, db *bbolt.DB) error {
	err := db.Update(func(tx *bbolt.Tx) error {
		timeBytes := make([]byte, 8)
		b := tx.Bucket([]byte("files"))
		binary.LittleEndian.PutUint64(timeBytes, uint64(time.Now().Unix()))
		return b.Put([]byte(path), timeBytes)
	})

	return errors.Wrap(err, "failed to update database")
}
