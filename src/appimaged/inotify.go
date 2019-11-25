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
	"github.com/probonopd/appimage/internal/helpers"
	"github.com/rjeczalik/notify"
	"log"
)

// Can we watch files with a certain file name extension only
// and how would this improve performance?

func inotifyWatch(path string) {
	// Make the channel buffered to ensure no event is dropped. Notify will drop
	// an event if the receiver is not able to keep up the sending pace.
	c := make(chan notify.EventInfo, 1)

	// Set up a watchpoint listening for inotify-specific events within a
	// current working directory. Dispatch each InCloseWrite and InMovedTo
	// events separately to c.
	if err := notify.Watch(path, c, notify.InCloseWrite, notify.InMovedTo, notify.InMovedFrom, notify.InDelete, notify.InDeleteSelf); err != nil {
		log.Fatal(err)
	}
	defer notify.Stop(c)

	for {
		// Block until an event is received.
		switch ei := <-c; ei.Event() {
		case notify.InDeleteSelf:
			log.Println("TODO:", ei.Path(), "was deleted, un-integrate all AppImages that were conteined herein")
			ToBeIntegratedOrUnintegrated = helpers.AppendIfMissing(ToBeIntegratedOrUnintegrated, ei.Path())
			// log.Println("ToBeIntegratedOrUnintegrated now contains:", ToBeIntegratedOrUnintegrated)
		default:
			log.Println("inotifyWatch:", ei.Path(), ei.Event())
			ToBeIntegratedOrUnintegrated = helpers.AppendIfMissing(ToBeIntegratedOrUnintegrated, ei.Path())
			// log.Println("ToBeIntegratedOrUnintegrated now contains:", ToBeIntegratedOrUnintegrated)
		}
	}
}

/*
// Watch a directory using inotify
func inotifyWatch(path string) {
	watcher, err := fsnotify.NewWatcher()
	helpers.LogError("inotify, probably already watching", err)

	if err == nil {

		err = watcher.Add(path)
		if err != nil {
			helpers.PrintError("inotify: watcher.Add", err)
		}
		log.Println("inotify: Watching", path)

		var done bool

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					if done {
						return
					}
					done = true
					continue
				}

				if *verbosePtr == true {
					log.Println("inotify:", event.Op, event.Name)
				}

				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Chmod == fsnotify.Chmod {
					// log.Println("inotify: Should check whether to register file:", event.Name)
					// We would be interesting in "write complete", file closed
					// IN_CLOSE https://stackoverflow.com/questions/2895187/which-inotify-event-signals-the-completion-of-a-large-file-operation
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						var dirs []string
						dirs = append(dirs, event.Name)
						watchDirectoriesReally(dirs) // If a directory has been created, watch that directory as well
					} else {
						ToBeIntegratedOrUnintegrated = helpers.AppendIfMissing(ToBeIntegratedOrUnintegrated, event.Name)
					}
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
					// log.Println("inotify: Should check whether to unregister file:", event.Name)
					// May want to check filesystem whether it was integrated at all before doing anything
					ToBeIntegratedOrUnintegrated = helpers.AppendIfMissing(ToBeIntegratedOrUnintegrated, event.Name)
					log.Println("inotify: TODO: If it was a directory (too late to find out), then also check if AppImages were in", event.Name, "that need to be unintegrated")
					// TODO: When a directory is deleted, we need to find all applications that
					// live inside that directory. Maybe we need to parse the already-installed desktop files
					// to find those efficiently
				}
			}
		}
	}
}
*/
