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

func handle(event fsnotify.Event) {

	// Before we do any action, we should probably put
	// the file into a queue in which it stays for some time
	// before we take any action on it, because one downloaded
	// file may trigger many inotify events and there seems to be
	// nothing like "download complete" besides IN_CLOSE which
	// I didn't find in fsnotify yet

	if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Chmod == fsnotify.Chmod {
		log.Println("watcher: Should check whether to register file:", event.Name)
		// We would be interesting in "write complete", file closed
		// IN_CLOSE https://stackoverflow.com/questions/2895187/which-inotify-event-signals-the-completion-of-a-large-file-operation
	}
	if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
		log.Println("watcher: Should check whether to unregister file:", event.Name)
		// May want to check filesystem whether it was integrated at all before doing anything
	}

}
