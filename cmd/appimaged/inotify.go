package main

// Watches a directory using inotify recursively.
// There is a package "recwatcher" but it only is recursive for the directories
// that were existing at the time when the watch was started, so it is not useful for us.
// So we handle recursion ourselves.
//
// We should find a way to do with inotify since
// normal users can only have 128 watched directories
// (subdirectories seemingly count separately)
// me@host:~$ cat /proc/sys/fs/inotify/max_user_instances
// 128
//
// TODO: Check https://github.com/amir73il/fsnotify-utils/wiki/Super-block-root-watch
// Super block root watch is designed to solve the scalability issues with inotify recursive watches.
// The fanotify super block watch patch set is meant to fill this gap in functionality and add the functionality of a root watch.
// It was merged to kernel v5.1-rc1.
// Currently fanotify needs root rights (CAP_ADMIN privileges)

// Not using the "gopkg.in/fsnotify.v1" package because it does not implement
// a way to find out when a complete is completed, since the needed IN_CLOSE_WRITE
// us Unix specific and not cross-platform. Therefore, we are using https://github.com/rjeczalik/notify

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/probonopd/go-appimage/src/goappimage"
)

var watcher *fsnotify.Watcher

// Add the directory to the directory watcher. Also initializes the watcher if it hasn't been used yet.
func AddWatchDir(dir string) (err error) {
	if watcher == nil {
		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(dir) {
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
	}
	// Adding a dir that's already added causes an error, so we make sure it's not watched to prevent this.
	for _, watched := range watcher.WatchList() {
		if watched == dir {
			return nil
		}
	}
	return watcher.Add(dir)
}

func RemoveWatchDir(dir string) {
	watcher.Remove(dir)
}

// Starts actually waiting for filesystem events. This will be run in a goroutine, so we don't want to return anything.
func StartWatch() {
mainLoop:
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("fsnotify event:", ev)
			for _, dir := range watchedDirectories {
				// Deleting or creating a watched directory gives fsnotify.Rename.
				if dir == ev.Name && ev.Has(fsnotify.Rename) {
					if helpers.Exists(dir) {
						AddIntegrationsFromDir(dir)
					} else {
						RemoveIntegrationsFromDir(dir)
					}
					continue mainLoop
				}

			}
			if !IsPossibleAppImage(ev.Name) {
				continue
			}
			if ev.Has(fsnotify.Write) {
				// Many write operations could be sent for the same file in a short amount of time.
				// writeWait will keep track of things and take appropriate actions when writing is done.
				writeWait(ev.Name)
				continue
			}
			_, ok = integrations[ev.Name]
			if ok {
				if ev.Has(fsnotify.Rename) || ev.Has(fsnotify.Remove) {
					// TODO: Add UpdateIntegration for renames instead of removing and readding it.
					RemoveIntegration(ev.Name, true)
				}
				continue
			}
			if ev.Has(fsnotify.Create) {
				// A create signal may be followed by write signals, so we wait for them to come through (if they do)
				writeWait(ev.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			helpers.PrintError("watcher", err)
		}
	}
}

// Only pay attention to files that are probably an appimage based on it's extention.
func IsPossibleAppImage(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".appimage") || strings.HasSuffix(strings.ToLower(path), ".app")
}

var timers = make(map[string]*time.Timer)

// Largely from https://github.com/fsnotify/fsnotify/blob/v1.6.0/cmd/fsnotify/dedup.go
func writeWait(path string) {
	t, ok := timers[path]
	if !ok {
		timers[path] = time.AfterFunc(100*time.Millisecond, func() { writeEnd(path) })
	} else {
		t.Reset(100 * time.Millisecond)
	}
}

func writeEnd(path string) {
	// We make sure the path is an AppImage in the first place
	_, ok := integrations[path]
	if goappimage.IsAppImage(path) {
		if !ok {
			AddIntegration(path, true)
		}
	} else if ok {
		RemoveIntegration(path, true)
	}
}
