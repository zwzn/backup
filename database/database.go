package database

import (
	"encoding/binary"
	"time"

	"github.com/abibby/backup/backend"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type DB struct {
	db *bbolt.DB
}

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0644, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database")
	}

	return &DB{
		db: db,
	}, err
}

func (db *DB) InitializeBackends(backends []backend.Backend) error {
	err := db.db.Update(func(tx *bbolt.Tx) error {
		for _, b := range backends {
			_, err := tx.CreateBucketIfNotExists([]byte(b.URI()))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to create database bucket")
	}
	return nil
}

func (db *DB) GetUpdatedTime(b backend.Backend, path string) (int64, error) {
	var t int64
	err := db.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(b.URI()))
		by := b.Get([]byte(path))

		if by != nil {
			t = int64(binary.LittleEndian.Uint64(by))
		}

		return nil
	})

	return t, errors.Wrap(err, "failed to read database")
}

func (db *DB) SetUpdatedTime(b backend.Backend, path string, t time.Time) error {
	err := db.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(b.URI()))
		return SetUpdatedTime(bucket, path, t)
	})

	return errors.Wrap(err, "failed to update database")
}

func SetUpdatedTime(bucket *bbolt.Bucket, path string, t time.Time) error {
	timeBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timeBytes, uint64(t.Unix()))
	return bucket.Put([]byte(path), timeBytes)
}

func (db *DB) Update(b backend.Backend, callback func(tx *bbolt.Tx, bucketName []byte) error) error {
	return db.db.Update(func(tx *bbolt.Tx) error {
		return callback(tx, []byte(b.URI()))
	})
}
