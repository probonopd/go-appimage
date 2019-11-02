package main

// Watches a directory using inotify.
//
// TODO: Check https://github.com/amir73il/fsnotify-utils/wiki/Super-block-root-watch
// Super block root watch is designed to solve the scalability issues with inotify recursive watches.
// The fanotify super block watch patch set is meant to fill this gap in functionality and add the functionality of a root watch.
// It was merged to kernel v5.1-rc1.

import (
	"log"

	"github.com/purpleidea/mgmt/recwatch"
	"gopkg.in/fsnotify.v1"
)

// Can we watch files with a certain file name extension only
// and how would this improve performance?

func inotifyWatch(path string) {
	watcher, err := recwatch.NewRecWatcher(path, true)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	log.Println("inotify: Watching", path)

	var done bool

	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				if done {
					return
				}
				done = true
				continue
			}
			if err := event.Error; err != nil {
				return
			}

			// log.Println("inotify:", event.Body.Op, event.Body.Name)

			if event.Body.Op&fsnotify.Write == fsnotify.Write || event.Body.Op&fsnotify.Create == fsnotify.Create {
				// log.Println("inotify: Should check whether to register file:", event.Body.Name)
				// We would be interesting in "write complete", file closed
				// IN_CLOSE https://stackoverflow.com/questions/2895187/which-inotify-event-signals-the-completion-of-a-large-file-operation
				toBeIntegrated = appendIfMissing(toBeIntegrated, event.Body.Name)
			}
			if event.Body.Op&fsnotify.Remove == fsnotify.Remove || event.Body.Op&fsnotify.Rename == fsnotify.Rename {
				// log.Println("inotify: Should check whether to unregister file:", event.Body.Name)
				// May want to check filesystem whether it was integrated at all before doing anything
				toBeUnintegrated = appendIfMissing(toBeUnintegrated, event.Body.Name)
			}
		}
	}
}

func appendIfMissing(slice []string, s string) []string {
	for _, ele := range slice {
		if ele == s {
			return slice
		}
	}
	return append(slice, s)
}
