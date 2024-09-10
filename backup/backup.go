package backup

import (
	"errors"
	"fmt"
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

type File struct {
	Path     string
	Modified time.Time
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
	wg.Add(3)

	files := stack.NewSyncDone[File]()
	fileQueue := make(chan File, 16)
	fileComplete := make(chan struct{}, 16)
	backupDone := make(chan struct{})

	var scanError error
	go func() {
		defer wg.Done()
		scanError = scanFolder(dir, db, o, fileQueue)
		files.Finish(true)
	}()

	var backupError error
	go func() {
		defer wg.Done()
		backupError = backupFiles(db, o, files, fileComplete)
		backupDone <- struct{}{}
	}()

	go func() {
		defer wg.Done()
		total := 0
		done := 0
		ticker := time.NewTicker(time.Second)
		start := time.Now()
		for {
			select {
			case f := <-fileQueue:
				files.Push(f)
				total++
			case <-fileComplete:
				done++
			case <-backupDone:
				return
			case now := <-ticker.C:
				runTime := now.Sub(start)
				remaining := total - done
				var timePerFile time.Duration
				if done > 0 {
					timePerFile = runTime / time.Duration(done)
				}
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
	close(backupDone)
	close(fileQueue)
	close(fileComplete)

	return errors.Join(scanError, backupError)
}
func scanFolder(dir string, db *database.DB, o *Options, filesChan chan File) error {
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
			// ignore symlinks
		} else {
			info, err := f.Info()
			if err != nil {
				slog.Error("failed to queue file", "file", p, "err", err)
				continue
			}
			filesChan <- File{
				Path:     p,
				Modified: info.ModTime(),
			}
		}
	}
	return nil
}

func needsUpdate(db *database.DB, b backend.Backend, f File) (bool, error) {
	updatedTime, err := db.GetUpdatedTime(b, f.Path)
	if err != nil {
		return false, err
	}

	if updatedTime >= f.Modified.Unix() {
		return false, nil
	}

	return true, nil
}
func backupFiles(db *database.DB, o *Options, files *stack.SyncDoneStack[File], done chan struct{}) error {
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

func backupFile(db *database.DB, b backend.Backend, f File) error {
	update, err := needsUpdate(db, b, f)
	if err != nil {
		return err
	}
	if !update {
		return nil
	}
	file, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	slog.Debug("back up file", "file", f.Path)
	err = b.Write(f.Path, info.ModTime(), file)
	if err != nil {
		return err
	}

	err = db.SetUpdatedTime(b, f.Path, info.ModTime())
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
