package main

// Currently not used at all since we are not using inotify at the moment.
// Watches a directory using inotify.
// Not clear yet whether we want to make use of this at all
// and if so then sparingly because it is said to eat resources
// (a claim still to be verified)

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

// Can we watch files with a certain file name extension only
// and how would this improve performance?

// https://developer.gnome.org/notification-spec/ has
// "transfer.complete": Completed file transfer
// Maybe we should watch those in addition to/instead of
// inotify?
// Are there notifications for folders being "looked at"?

func NewWatcher(path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				handle(event)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Watching", path)

}
