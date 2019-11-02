package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/procfs"

	"github.com/adrg/xdg"
)

// TODO: Understand whether we can make clever use of
// org.freedesktop.thumbnails.Thumbnailer1 dbus
// rather than (or in addition to) using inotify at all

// Can register a specialized thumbnailer using
// the org.freedesktop.thumbnails.Manager1 interface
// then it is active as long as the thumbnailer service is running
// Or install a system-wide (or per user in $XDG_DATA_DIRS) dbus service file, e.g.,
// /usr/share/dbus-1/services/org.gtk.vfs.Daemon.service
// This launches the service when need it

var quit = make(chan struct{})

var overwritePtr = flag.Bool("o", false, "Overwrite existing desktop integration")
var cleanPtr = flag.Bool("c", false, "Clean pre-existing desktop files")
var quietPtr = flag.Bool("q", false, "Do not send desktop notifications")

var toBeIntegrated []string
var toBeUnintegrated []string

func main() {

	log.Println("main: Running from", here())
	log.Println("main: xdg.DataHome =", xdg.DataHome)

	checkPrerequisites()

	flag.Parse()
	log.Println("Overwrite:", *overwritePtr)
	log.Println("Clean:", *overwritePtr)

	// Disable desktop integration provided by scripts within AppImages
	// as per https://github.com/AppImage/AppImageSpec/blob/master/draft.md#desktop-integration
	os.Setenv("DESKTOPINTEGRATION", "go-appimaged")

	// Maybe not needed? At least on Xubuntu it seems to work without this
	// Try to register ourselves as a thumbnailer for AppImages, in the hope that
	// DBus notifications will be generated for AppImages as thumbnail-able files
	// FIXME: Currently getting: No such interface 'org.freedesktop.thumbnails' on object at path /org/freedesktop/thumbnails/Manager1
	// conn, err := dbus.SessionBus()
	// if err != nil {
	// 	panic(err)
	// }
	// obj := conn.Object("org.freedesktop.thumbnails.Manager1", "/org/freedesktop/thumbnails/Manager1")
	// call := obj.Call("org.freedesktop.thumbnails.Manager1", 0, "", uint32(0),
	// 	"Register", "DBP_DBUS_THUMB_OBJECT", "file", "application/vnd.appimage", []string{},
	// 	map[string]dbus.Variant{}, int32(5000))
	// if call.Err != nil {
	// 	println(call.Err.Error())
	// }

	// Register AppImages from well-known locations
	// https://github.com/AppImage/appimaged#monitored-directories specifies:
	// $HOME/Downloads (or its localized equivalent, as determined by G_USER_DIRECTORY_DOWNLOAD in glib)
	// $HOME/.local/bin
	// $HOME/bin
	// $HOME/Applications
	// /Applications
	// [any mounted partition]/Applications
	// /opt
	// /usr/local/bin

	home, _ := os.UserHomeDir()
	// FIXME: Use XDG translated names for Downloads and Desktop; blocked by https://github.com/adrg/xdg/issues/1 or https://github.com/OpenPeeDeeP/xdg/issues/6
	watchedDirectories := []string{
		home + "/Downloads",
		home + "/Desktop",
		home + "/.local/bin",
		home + "/bin",
		home + "/Applications",
		home + "/opt",
		home + "/usr/local/bin",
	}

	mounts, _ := procfs.GetMounts()
	// FIXME: This breaks when the partition label has "-", see https://github.com/prometheus/procfs/issues/227

	for _, mount := range mounts {
		log.Println(mount.MountPoint)
		if strings.HasPrefix(mount.MountPoint, "/sys") == false && // Is /dev needed for openSUSE Live?
			strings.HasPrefix(mount.MountPoint, "/run") == false &&
			strings.HasPrefix(mount.MountPoint, "/tmp") == false &&
			strings.HasPrefix(mount.MountPoint, "/proc") == false {
			watchedDirectories = append(watchedDirectories, mount.MountPoint+"/Applications")
		}
	}

	// TODO: Maybe we don't want to walk subdirectories?
	// filepath.Walk is handy but scans subfolders too, by default, which might not be what you want.
	// The Go stdlib also provides ioutil.ReadDir
	println("Registering AppImages in well-known locations and their subdirectories...")
	println("TODO: Use all mounted disks; react to disks coming and going using UDisks2")

	for _, v := range watchedDirectories {
		err := filepath.Walk(v, func(path string, info os.FileInfo, err error) error {

			if err != nil {
				// log.Printf("%v\n", err)
			} else if info.IsDir() == true {
				go inotifyWatch(path)
			} else if info.IsDir() == false {
				ai := newAppImage(path)
				if ai.imagetype > 0 {
					go ai.integrate()
				}
			}

			return nil
		})
		if err != nil {
			log.Printf("error walking the path %q: %v\n", v, err)
			return
		}
	}

	// Use dbus to find out about AppImages to be handled
	// go monitorDbusSessionBus()
	// or
	// use inotify to find out about AppImages to be handled

	// sendDesktopNotification("watcher", "Started")

	// Ticker to periodically move desktop files into system
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				moveDesktopFiles()

			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	<-quit

}

// Periodically move desktop files from their temporary location
// into the menu, so that the menu does not get rebuilt all the time
func moveDesktopFiles() {
	// log.Println("main: Ticktock")

	for _, path := range toBeIntegrated {
		ai := newAppImage(path)
		go ai.integrate()
	}
	toBeIntegrated = nil

	for _, path := range toBeUnintegrated {
		ai := newAppImage(path)
		go ai.removeIntegration()
	}
	toBeUnintegrated = nil

	desktopcachedir := xdg.CacheHome + "/applications/" // FIXME: Do not hardcode here and in other places

	files, err := ioutil.ReadDir(desktopcachedir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {

		log.Println("main: Moving", file.Name(), "to", xdg.DataHome+"/applications/")
		err = os.Rename(desktopcachedir+"/"+file.Name(), xdg.DataHome+"/applications/"+file.Name())
		if err != nil {
			log.Println(err)
		}
	}

	// Use desktop-file-install instead?

	// var filenames []string
	// for _, file := range files {
	// 	filenames = append(filenames, file.Name())
	// }
	// cmd := "desktop-file-install"
	// if err := exec.Command(cmd, filenames...).Run(); err != nil {
	// 	printError("main: desktop-file-install", err)
	// }

}

// Returns the location of the executable
func here() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
		return ""
	}
	return (dir)
}

// Print error, prefixed by a string that explains the context
func printError(context string, e error) {
	if e != nil {
		log.Println("ERROR", context+":", e)
	}
}
