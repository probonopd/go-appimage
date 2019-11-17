package main

// Monitors the DBus session bus for relevant messages in the
// /org/freedesktop/thumbnails namespace.
// It is not clear yet whether those are sufficient for everything
// we want to monitor, or whether we will need to watch/monitor
// additional things
// TODO: Watch for browser download completions
// http://www.galago-project.org/specs/notification/0.9/x211.html
// TODO: Firefox end-of-download triggers a notification
// (Chrome does NOT seem to)
// dbus-monitor 2>%1 | grep "path=/org/gtk/vfs/metadata" -C 10
// TODO: KDE doesn't send /org/freedesktop/thumbnails
// but does send interface=org.kde.KDirNotify and interface=org.kde.JobViewV2
// and interface=org.kde.ActivityManager.ResourcesScoring

// https://developer.gnome.org/notification-spec/ has
// "transfer.complete": Completed file transfer
// Maybe we should watch those in addition to/instead of
// inotify?
// Are there notifications for folders being "looked at"?

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-language-server/uri"
	"github.com/godbus/dbus"
	"github.com/probonopd/appimage/internal/helpers"
)

func removeDuplicatesUnordered(elements []string) []string {
	encountered := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	var result []string
	for key := range encountered {
		result = append(result, key)
	}
	return result
}

// The session bus is for your login (e.g. desktop notifications), while the
// system bus handles system-wide stuff (e.g. USB stick plugged in).
// TODO: Watch system bus, too.

func monitorDbusSessionBus() {

	conn, err := dbus.SessionBusPrivate() // When using SessionBusPrivate(), need to follow with Auth(nil) and Hello()
	defer conn.Close()
	if err != nil {
		helpers.PrintError("SessionBusPrivate", err)
		return
	}
	if conn == nil {
		helpers.PrintError("No conn", err)
		return
	}

	if err = conn.Auth(nil); err != nil {
		helpers.PrintError("Auth", err)
		return
	}

	if err = conn.Hello(); err != nil {
		conn.Close()
		helpers.PrintError("Hello", err)
		return
	}

	var rules = []string{
		// "path_namespace='/'", // Everything
		"path_namespace='/org/freedesktop/thumbnails'",
		"interface='org.kde.ActivityManager.ResourcesScoring'",
		"interface='org.kde.KDirNotify'",
		"interface='org.kde.JobViewV2'",
		"interface='org.gtk.vfs.Metadata', member='Set'", // This notifies us e.g., of finished Firefox downloads
		// "interface='org.freedesktop.UDisks2'"
	}
	var flag uint = 0

	call := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, flag)
	if call.Err != nil {
		log.Println(os.Stderr, "Failed to become monitor:", call.Err)
		return
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)

	log.Println("monitor: Monitoring DBus session bus")

	// Note:
	// https://godoc.org/github.com/godbus/dbus says:
	// For incoming messages, the inverse of these rules are used, with the
	// exception of STRUCTs. Incoming STRUCTS are represented as a slice of
	// empty interfaces containing the struct fields in the correct order.
	// The Store function can be used to convert such values to Go structs.
	//
	// Whatever this means. Probably we are doing it wrong below.
	// https://godoc.org/github.com/godbus/dbus#Store is really not that
	// insightful either. An example would be appreciated.

	// TODO: Check https://golang.hotexamples.com/de/examples/github.com.godbus.dbus/Conn/BusObject/golang-conn-busobject-method-examples.html
	// examples on how to react to messages
	// Noteworthy: s.Body[2].(string)

	for v := range c {
		// log.Println(v.Headers)
		if v.Headers[2].Value() == "org.gtk.vfs.Metadata" {
			// log.Println("Something is going on in VFS Metadata")
			// log.Println("# v.Headers:", v.Headers)
			// log.Println("# v.Body:", v.Body)
			str := fmt.Sprintf("%s", v.Body[1]) // TODO: There must be a better way to turn a []byte into a string
			str = str[:len(str)-1]              // TODO: dito
			if strings.HasPrefix(str, "%!s") == false {
				log.Println("org.gtk.vfs.Metadata", str)
				// time.Sleep(1 * time.Second)
				ai := NewAppImage(str)
				go ai.IntegrateOrUnintegrate()
			}

		}

		// log.Println("# v.Headers[3]:", v.Headers[3]) // Cleanup, Copy, Move, Queue, Dequeue, Started, Finished, Error

		// TODO: Why does KDE not seem to send /org/freedesktop/thumbnails messages?

		// KDE
		// org.kde.ActivityManager.ResourcesScoring
		// This catches files downloaded in KDE using Firefox
		// but not files copied in the file manager;
		// TODO: we need to catch those with FilesAdded
		if v.Headers[3].String() == "\"ResourceScoreUpdated\"" {
			fp := v.Body[2].(string)
			log.Println("monitor: ResourceScoreUpdated: ", fp)
			ai := NewAppImage(fp)
			go ai.IntegrateOrUnintegrate()
		}

		// KDE
		// org.kde.KDirNotify
		// This catches local file operations in the filemanager
		// TODO: Implement
		if v.Headers[3].String() == "\"FileMoved\"" {
			log.Println("monitor: FileMoved: Not implemented yet")
		}
		if v.Headers[3].String() == "\"FilesAdded\"" {
			log.Println("monitor: FilesAdded: Not implemented yet")
			log.Println("The trouble is that we don't know the path of the added file, only its directory!")
			log.Println("monitor:", v.Body[0])
		}
		if v.Headers[3].String() == "\"FilesRemoved\"" {
			log.Println("monitor: FilesRemoved: Not implemented yet")
		}

		if v.Headers[3].String() == "\"Queue\"" {
			if len(v.Body) >= 2 {
				files := v.Body[0].([]string) // Workaround for: cannot range over v.Body[0] (type interface {})
				mimetypes := v.Body[1].([]string)
				for i, s := range files {
					if mimetypes[i] == "application/vnd.appimage" || mimetypes[i] == "application/x-iso9660-appimage" {
						fp := getFilepath(s)
						log.Println("monitor: Queue: ", fp)
						// ai := newAppImage(fp)
						// setExecutable(ai) // TODO: Unlear whether we need to do something here
					}
				}
				// log.Println("v.Body[1]:", v.Body[1]) // [application/vnd.appimage] or [application/x-iso9660-appimage]

			}
		}

		if v.Headers[3].String() == "\"Move\"" {
			if len(v.Body) >= 2 {
				fromfiles := removeDuplicatesUnordered(v.Body[0].([]string))
				tofiles := removeDuplicatesUnordered(v.Body[1].([]string))
				for _, s := range fromfiles {
					fp := getFilepath(s)
					log.Println("monitor: MoveFrom: ", fp)
					ai := NewAppImage(fp)
					ai.IntegrateOrUnintegrate()
				}
				for _, s := range tofiles {
					fp := getFilepath(s)
					log.Println("monitor: MoveTo: ", fp)
					ai := NewAppImage(fp)
					go ai.IntegrateOrUnintegrate()
				}
			}
		}

		if v.Headers[3].String() == "\"Copy\"" {
			if len(v.Body) >= 2 {
				fromfiles := removeDuplicatesUnordered(v.Body[0].([]string))
				tofiles := removeDuplicatesUnordered(v.Body[1].([]string))
				for _, s := range fromfiles {
					fp := getFilepath(s)
					log.Println("monitor: CopyFrom: ", fp)
					// setExecutable(fp) // TODO: If we do that, shouldn't we search the whole directory for AppImages?
				}
				for _, s := range tofiles {
					fp := getFilepath(s)
					log.Println("monitor: CopyTo: ", fp)
					ai := NewAppImage(fp)
					go ai.IntegrateOrUnintegrate()
				}
			}
		}

		if v.Headers[3].String() == "\"Cleanup\"" || v.Headers[3].String() == "\"Delete\"" {
			if len(v.Body) >= 1 {
				files := removeDuplicatesUnordered(v.Body[0].([]string))
				for _, s := range files {
					fp := getFilepath(s)
					log.Println("monitor: CleanupOrDelete: ", fp)
					ai := NewAppImage(fp)
					ai.IntegrateOrUnintegrate()
				}
			}
		}

	}
}

func getFilepath(uristring string) string {
	// log.Println(uristring)

	if uristring[:7] != "file://" {
		log.Println("To be imlpemented:", uristring[:7])
		return ""
	}
	return uri.New(uristring).Filename()
}
