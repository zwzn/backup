package backup

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/abibby/backup/backend"
	"github.com/abibby/backup/database"
	"github.com/abibby/backup/stack"
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
		return fmt.Errorf("failed to initialize backends in local database: %w", err)
	}
	defer printTime(time.Now())

	var wg sync.WaitGroup
	wg.Add(2)

	files := stack.NewSyncDone[string]()
	filesChan := make(chan string, 16)
	doneChan := make(chan struct{}, 16)

	var scanError error
	go func() {
		scanError = scanFolder(dir, db, o, filesChan)
		files.Finish(true)
		wg.Done()
	}()

	var backupError error
	go func() {
		backupError = backupFiles(db, o, files, doneChan)
		wg.Done()
	}()

	go func() {
		total := 0
		done := 0
		ticker := time.NewTicker(time.Second)
		start := time.Now()
		for {
			select {
			case f := <-filesChan:
				files.Push(f)
				total++
			case <-doneChan:
				done++
			case now := <-ticker.C:
				runTime := now.Sub(start)
				remaining := total - done
				timePerFile := runTime / time.Duration(done)
				timeRemaining := timePerFile * time.Duration(remaining)
				endingAt := now.Add(timeRemaining)
				slog.Info("backing up",
					"total", total,
					"done", done,
					"remaining", timeRemaining.Truncate(time.Second),
					"end", endingAt,
				)
			}
		}
	}()

	wg.Wait()
	defer close(filesChan)

	return errors.Join(scanError, backupError)
}
func scanFolder(dir string, db *database.DB, o *Options, filesChan chan string) error {
	var err error
	files, err := os.ReadDir(dir)

	if err != nil {
		return fmt.Errorf("failed to load directory %s: %w", dir, err)
	}

	for _, f := range files {
		p := path.Join(dir, f.Name())
		if matches(p, o.Ignore) {
			continue
		}
		if f.IsDir() {
			err = scanFolder(p, db, o, filesChan)
			if err != nil {
				slog.Error("failed to backup file", "file", p, "err", err)
			}
		} else if f.Type()&os.ModeSymlink != 0 {
		} else {
			for _, b := range o.Backends {
				update, err := needsUpdate(db, b, p, f)
				if err != nil {
					slog.Error("failed to queue file", "file", p, "err", err)
					continue
				}
				if update {
					slog.Debug("queueing", "file", p, "backend", b.URI())
					filesChan <- p
				}

			}
		}
	}
	return nil
}

func needsUpdate(db *database.DB, b backend.Backend, p string, f fs.DirEntry) (bool, error) {
	updatedTime, err := db.GetUpdatedTime(b, p)
	if err != nil {
		return false, err
	}

	info, err := f.Info()
	if err != nil {
		return false, err
	}
	if updatedTime >= info.ModTime().Unix() {
		return false, nil
	}

	return true, nil
}
func backupFiles(db *database.DB, o *Options, files *stack.SyncDoneStack[string], done chan struct{}) error {
	for f := range files.All() {
		for _, backend := range o.Backends {
			err := backupFile(db, backend, f)
			if err != nil {
				slog.Error("failed to back up file", "file", f, "err", err)
			}
			done <- struct{}{}
		}
	}
	return nil
}

func backupFile(db *database.DB, b backend.Backend, p string) error {
	file, err := os.Open(p)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	slog.Debug("back up file", "file", p)
	err = b.Write(p, info.ModTime(), file)
	if err != nil {
		return err
	}

	err = db.SetUpdatedTime(b, p, info.ModTime())
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
