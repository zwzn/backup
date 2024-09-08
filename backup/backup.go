package backup

import (
	"io/ioutil"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/abibby/backup/backend"
	"github.com/abibby/backup/database"
	"github.com/pkg/errors"
)

type Options struct {
	Backends []backend.Backend
	Ignore   []string
}

func printTime(start time.Time) {
	duration := time.Since(start)
	if duration > time.Second {
		duration = duration.Truncate(time.Second)
	}
	if duration > time.Millisecond {
		duration = duration.Truncate(time.Millisecond)
	}
	slog.Info("Backup complete", "duration", duration)
}

func Backup(db *database.DB, dir string, o *Options) error {
	err := db.InitializeBackends(o.Backends)
	if err != nil {
		return errors.Wrap(err, "failed to initialize backends in local database")
	}
	defer printTime(time.Now())

	return backupFolder(dir, db, o)
}

func backupFolder(dir string, db *database.DB, o *Options) error {
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
				slog.Error("failed to backup file", "file", p, "err", err)
			}
		} else if f.Mode()&os.ModeSymlink != 0 {
		} else {
			slog.Debug("backing up", "file", p)
			for _, b := range o.Backends {
				err = backupFile(db, b, p, f)
				if err != nil {
					slog.Error("failed to backup file", "file", p, "err", err)
				}

			}
		}
	}
	return nil
}

func backupFile(db *database.DB, b backend.Backend, p string, f os.FileInfo) error {
	ut, err := db.GetUpdatedTime(b, p)
	if err != nil {
		return err
	}

	if ut >= f.ModTime().Unix() {
		return nil
	}
	t := time.Now()

	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer file.Close()

	err = b.Write(p, t, file)
	if err != nil {
		return err
	}

	err = db.SetUpdatedTime(b, p, t)
	if err != nil {
		return err
	}
	return nil
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
