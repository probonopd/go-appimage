package main

// Currently not used at all since we are not using inotify at the moment.
// Handles events coming from inotify.
// Maybe should be moved to watcher.go

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

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
