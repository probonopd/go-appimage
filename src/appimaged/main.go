package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/godbus/dbus"
	"github.com/prometheus/procfs"
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

var verbosePtr = flag.Bool("v", false, "Print verbose log messages")
var overwritePtr = flag.Bool("o", false, "Overwrite existing desktop integration")
var cleanPtr = flag.Bool("c", false, "Clean pre-existing desktop files")
var notifPtr = flag.Bool("n", false, "Send desktop notifications")
var noZeroconfPtr = flag.Bool("nz", false, "Do not announce this service on the network using Zeroconf")

var toBeIntegrated []string
var toBeUnintegrated []string

var conn *dbus.Conn

func main() {

	// As quickly as possible go there if we are invoked with the "appwrap" command
	if len(os.Args) > 1 {
		if os.Args[1] == "appwrap" {
			appwrap()
			os.Exit(0)
		}
	}

	flag.Parse()

	var err error
	// Catch for young players:
	// conn, err := dbus.SessionBus() would not work here,
	// https://stackoverflow.com/a/34195389
	conn, err = dbus.SessionBus()
	// defer conn.Close()
	if err != nil {
		log.Println("main: Failed to connect to session bus:", err)
		os.Exit(1)
	}
	if conn == nil {
		log.Println("ERROR: notification: Could not get conn")
		os.Exit(1)
	}

	if *notifPtr == true {
		sendDesktopNotification("Starting", here())
	}

	log.Println("main: Running from", here())

	// The directory we run from is added to the $PATH so that we find helper
	// binaries there, too
	os.Setenv("PATH", here()+":"+os.Getenv("PATH"))
	log.Println("main: PATH:", os.Getenv("PATH"))

	log.Println("main: xdg.DataHome =", xdg.DataHome)

	checkPrerequisites()
	deleteDesktopFilesWithNonExistingTargets()

	log.Println("Overwrite:", *overwritePtr)
	log.Println("Clean:", *overwritePtr)

	// Disable desktop integration provided by scripts within AppImages
	// as per https://github.com/AppImage/AppImageSpec/blob/master/draft.md#desktop-integration
	os.Setenv("DESKTOPINTEGRATION", "go-appimaged")

	// Try to register ourselves as a thumbnailer for AppImages, in the hope that
	// DBus notifications will be generated for AppImages as thumbnail-able files
	// FIXME: Currently getting: No such interface 'org.freedesktop.thumbnails' on object at path /org/freedesktop/thumbnails/Manager1
	// Maybe not needed? At least on Xubuntu it seems to work without this
	// but perhaps it is why KDE ignores our nice thumbnails

	// React to partitions being mounted and unmounted
	go monitorUdisks()

	watchDirectories()

	if *noZeroconfPtr == false {
		if checkIfConnectedToNetwork() == true {
			go registerZeroconfService()
		}
	}
	go browseZeroconfServices()

	// Use dbus to find out about AppImages to be handled
	// go monitorDbusSessionBus()
	// or
	// use inotify to find out about AppImages to be handled

	if *notifPtr == true {
		sendDesktopNotification("watcher", "Started")
	}

	// Ticker to periodically move desktop files into system
	ticker := time.NewTicker(2 * time.Second)
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
	// log.Println("main: xxxxxxxxxxxxxxx Ticktock")

	// log.Println(watchedDirectories)
	// for _, w := range watchedDirectories {
	// 	log.Println(w.Path)
	// }

	for _, path := range toBeIntegrated {
		ai := newAppImage(path)
		go ai.integrateOrUnintegrate()
	}
	toBeIntegrated = nil

	for _, path := range toBeUnintegrated {
		ai := newAppImage(path)
		go ai.integrateOrUnintegrate()
	}
	toBeUnintegrated = nil

	desktopcachedir := xdg.CacheHome + "/applications/" // FIXME: Do not hardcode here and in other places

	files, err := ioutil.ReadDir(desktopcachedir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if *verbosePtr == true {
			log.Println("main: Moving", file.Name(), "to", xdg.DataHome+"/applications/")
		}
		err = os.Rename(desktopcachedir+"/"+file.Name(), xdg.DataHome+"/applications/"+file.Name())
		printError("main", err)
	}
	if len(files) > 0 {
		log.Println("main: Moved", len(files), "desktop files to", xdg.DataHome+"/applications/; use -v to see details")

		// Run the various tools that make sure that the added desktop files really show up in the menu.
		// Of course, almost no 2 systems are similar.
		updateMenuCommands := []string{
			"update-menus", // Needed on Ubuntu MATE so that the menu gets populated
		}
		for _, updateMenuCommand := range updateMenuCommands {
			if isCommandAvailable(updateMenuCommand) {
				cmd := exec.Command(updateMenuCommand)
				err := cmd.Run()
				if err == nil {
					log.Println("Ran", updateMenuCommand, "command")
				} else {
					printError("main: "+updateMenuCommand, err)
				}
			}

		}

		// Run update-desktop-database
		// "Build cache database of MIME types handled by desktop files."
		if isCommandAvailable("update-desktop-database") {
			cmd := exec.Command("update-desktop-database", xdg.DataHome+"/applications/")
			err := cmd.Run()
			if err == nil {
				log.Println("Ran", "update-desktop-database "+xdg.DataHome+"/applications/")
			} else {
				printError("main", err)
			}
		}

		/*
			// Run xdg-desktop-menu forceupdate
			// It probably doesn't hurt, although it may not really be needed.
			if isCommandAvailable("xdg-desktop-menu") {
				cmd := exec.Command("xdg-desktop-menu", "forceupdate")
				err := cmd.Run()
				if err == nil {
					log.Println("Ran", "xdg-desktop-menu forceupdate")
				} else {
					printError("main", err)
				}
			}
		*/
	}
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

func watchDirectories() {

	// Register AppImages from well-known locations
	// https://github.com/AppImage/appimaged#monitored-directories
	home, _ := os.UserHomeDir()

	os.MkdirAll(home+"/Applications", 0755)

	// FIXME: Use XDG translated names for Downloads and Desktop; blocked by
	// https://github.com/adrg/xdg/issues/1 or https://github.com/OpenPeeDeeP/xdg/issues/6
	watchedDirectories := []string{
		home + "/Downloads", // TODO: XDG localized version
		home + "/Desktop",   // TODO: XDG localized version
		home + "/.local/bin",
		home + "/bin",
		home + "/Applications",
		home + "/opt",
		home + "/usr/local/bin",
	}

	mounts, _ := procfs.GetMounts()
	// FIXME: This breaks when the partition label has "-", see https://github.com/prometheus/procfs/issues/227

	for _, mount := range mounts {
		if *verbosePtr == true {
			log.Println("main: MountPoint", mount.MountPoint)
		}
		if strings.HasPrefix(mount.MountPoint, "/sys") == false && // Is /dev needed for openSUSE Live?
			strings.HasPrefix(mount.MountPoint, "/run") == false &&
			strings.HasPrefix(mount.MountPoint, "/tmp") == false &&
			strings.HasPrefix(mount.MountPoint, "/proc") == false {
			watchedDirectories = appendIfMissing(watchedDirectories, mount.MountPoint+"/Applications")
		}
	}
	// TODO: Maybe we don't want to walk subdirectories?
	// filepath.Walk is handy but scans subfolders too, by default, which might not be what you want.
	// The Go stdlib also provides ioutil.ReadDir
	log.Println("Registering AppImages in well-known locations and their subdirectories...")

	watchDirectoriesReally(watchedDirectories)

	deleteDesktopFilesWithNonExistingTargets()
	// So this should also catch AppImages which were formerly hidden in some subdirectory
	// where the whole directory was deleted
}

func watchDirectoriesReally(watchedDirectories []string) {
	for _, v := range watchedDirectories {
		err := filepath.Walk(v, func(path string, info os.FileInfo, err error) error {

			if err != nil {
				// log.Printf("%v\n", err)
			} else if info.IsDir() == true {
				go inotifyWatch(path)
			} else if info.IsDir() == false {
				ai := newAppImage(path)
				if ai.imagetype > 0 {
					go ai.integrateOrUnintegrate()
				}
			}

			return nil
		})
		printError("main: watchDirectoriesReally", err)
	}
}
