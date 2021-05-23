package reconcile

import (
	"path/filepath"

	"github.com/abibby/backup/backend"
	"github.com/abibby/backup/database"
	"go.etcd.io/bbolt"
)

func Reconcile(db *database.DB, b backend.Backend) error {
	return db.Update(b, func(tx *bbolt.Tx, bucketName []byte) error {
		err := tx.DeleteBucket(bucketName)
		if err != nil {
			return err
		}

		bucket, err := tx.CreateBucket(bucketName)
		if err != nil {
			return err
		}
		return reconcile(bucket, b, "/")
	})
}

func reconcile(bucket *bbolt.Bucket, b backend.Backend, path string) error {
	files, err := b.List(path)
	if err != nil {
		return err
	}

	for _, f := range files {
		fullPath := filepath.Join(path, f.Name())
		if f.IsDir() {
			reconcile(bucket, b, fullPath)
		} else {
			versions := f.Versions()
			database.SetUpdatedTime(bucket, fullPath, versions[len(versions)-1])
		}
	}
	return nil
}
