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

	"github.com/rjeczalik/notify"
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
	if err := notify.Watch(path, c, notify.InCloseWrite, notify.InMovedTo,
		notify.InMovedFrom, notify.InDelete,
		notify.InDeleteSelf); err != nil {
		log.Println(err) // Don't be fatal if a directory cannot be read (e.g., no read rights)
	}
	defer notify.Stop(c)

	for {
		// Block until an event is received.
		switch ei := <-c; ei.Event() {
		case notify.InDeleteSelf:
			log.Println("TODO:", ei.Path(), "was deleted, un-integrate all AppImages that were conteined herein")
			integrationChannel <- ei.Path()
			// log.Println("ToBeIntegratedOrUnintegrated now contains:", ToBeIntegratedOrUnintegrated)
		default:
			log.Println("inotifyWatch:", ei.Path(), ei.Event())
			integrationChannel <- ei.Path()
			// log.Println("ToBeIntegratedOrUnintegrated now contains:", ToBeIntegratedOrUnintegrated)
		}
	}
}
