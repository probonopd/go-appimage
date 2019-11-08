package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus"
	helpers "github.com/probonopd/appimage/internal/helpers"
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
var quietPtr = flag.Bool("q", false, "Do not send desktop notifications")
var noZeroconfPtr = flag.Bool("nz", false, "Do not announce this service on the network using Zeroconf")

var toBeIntegrated []string
var toBeUnintegrated []string

var thisai AppImage // A reference to myself

var conn *dbus.Conn
var MQTTclient mqtt.Client

// This key in the desktop files written by us describes where the AppImage is in the filesystem.
// We need this because we rewrite Exec= to include things like wrap and Firejail
const ExecLocationKey = helpers.ExecLocationKey

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

func main() {

	// As quickly as possible go there if we are invoked with the "notify" command
	if len(os.Args) > 1 {
		if os.Args[1] == "notify" {
			JustNotify()
			os.Exit(0)
		}
	}

	// As quickly as possible go there if we are invoked with the "wrap" command
	if len(os.Args) > 1 {
		if os.Args[1] == "wrap" {
			appwrap()
			os.Exit(0)
		}
	}

	flag.Parse()

	// Always show version
	if commit != "" {
		fmt.Printf("%s %s\n", filepath.Base(os.Args[0]), commit)
	} else {
		fmt.Println("Unsupported local", filepath.Base(os.Args[0]), "developer build")
	}

	checkPrerequisites()

	// Connect to MQTT server and subscribe to the topic for ourselves
	if CheckIfConnectedToNetwork() == true {
		uri, err := url.Parse(helpers.MQTTServerURI)
		if err != nil {
			log.Fatal(err)
		}

		// go SubscribeMQTT(MQTTclient, "gh-releases-zsync|probonopd|merkaartor|continuous|Merkaartor-*-x86_64.AppImage.zsync")
		// go SubscribeMQTT(MQTTclient, "gh-releases-zsync|AppImage|AppImageKit|continuous|appimagetool-x86_64.AppImage.zsync")

		MQTTclient = connect("sub", uri)
		log.Println("MQTT client connected:", MQTTclient.IsConnected())
		if thisai.imagetype > 0 {
			go SubscribeMQTT(MQTTclient, thisai.updateinformation)
		}
	}

	// If we are running from an AppImage,
	// we subscribe to this application's topic, and we handle messages for this topic in a special way
	if thisai.imagetype > 0 {
		go SubscribeMQTT(MQTTclient, thisai.updateinformation)
	}

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

	// SimpleNotify("Starting", helpers.Here(), 5000)

	log.Println("main: Running from", helpers.Here())

	helpers.AddHereToPath()

	log.Println("main: xdg.DataHome =", xdg.DataHome)

	helpers.DeleteDesktopFilesWithNonExistingTargets()

	log.Println("Overwrite:", *overwritePtr)
	log.Println("Clean:", *overwritePtr)

	// Disable desktop integration provided by scripts within AppImages
	// as per https://github.com/AppImage/AppImageSpec/blob/master/draft.md#desktop-integration
	os.Setenv("DESKTOPINTEGRATION", "go-appimaged")

	// TODO: Also react to network interfaces and network connections coming and going,
	// refer to the official NetworkManager dbus specification:
	// https://developer.gnome.org/NetworkManager/1.16/spec.html
	if *noZeroconfPtr == false {
		if CheckIfConnectedToNetwork() == true {
			go registerZeroconfService()
			go browseZeroconfServices()
		}
	}

	// Try to register ourselves as a thumbnailer for AppImages, in the hope that
	// DBus notifications will be generated for AppImages as thumbnail-able files
	// FIXME: Currently getting: No such interface 'org.freedesktop.thumbnails' on object at path /org/freedesktop/thumbnails/Manager1
	// Maybe not needed? At least on Xubuntu it seems to work without this
	// but perhaps it is why KDE ignores our nice thumbnails

	// React to partitions being mounted and unmounted
	go monitorUdisks()

	watchDirectories()

	// Ticker to periodically check whether MQTT is still connected.
	// Periodically check whether the MQTT client is
	// still connected; try to reconnect if it is not.
	// This is recommended by MQTT servers since they can go
	// down for maintenance
	ticker2 := time.NewTicker(120 * time.Second)
	go func() {
		for {
			select {
			case <-ticker2.C:
				checkMQTTConnected(MQTTclient)
			case <-quit:
				ticker2.Stop()
				return
			}
		}
	}()

	// Ticker to periodically move desktop files into system
	ticker := time.NewTicker(2 * time.Second)
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

	quit := make(chan struct{})

	<-quit

}

// checkMQTTConnected checks whether the MQTT client is
// still connected; try to reconnect if it is not.
// This is recommended by MQTT servers since they can go
// down for maintenance
func checkMQTTConnected(MQTTclient mqtt.Client) {
	if CheckIfConnectedToNetwork() == true {
		if MQTTclient.IsConnected() == false {
			log.Println("MQTT client connected:", MQTTclient.IsConnected())
			MQTTclient.Connect()
			log.Println("MQTT client connected:", MQTTclient.IsConnected())
			// TODO: Do we need to subscribe everything again when this happens?
			// Not if we use a persistent session, see
			// https://www.hivemq.com/blog/mqtt-essentials-part-7-persistent-session-queuing-messages/
			// TODO: use a persistent session with the appropriate quality of service level
		}
	}
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
		helpers.LogError("main", err)
	}
	if len(files) > 0 {

		if *verbosePtr == true {
			log.Println("main: Moved", len(files), "desktop files to", xdg.DataHome+"/applications/")
		} else {
			log.Println("main: Moved", len(files), "desktop files to", xdg.DataHome+"/applications/; use -v to see details")
		}

		SimpleNotify("Added "+strconv.Itoa(len(files))+" applications", "", 5000)

		// Run the various tools that make sure that the added desktop files really show up in the menu.
		// Of course, almost no 2 systems are similar.
		updateMenuCommands := []string{
			"update-menus", // Needed on Ubuntu MATE so that the menu gets populated
		}
		for _, updateMenuCommand := range updateMenuCommands {
			if helpers.IsCommandAvailable(updateMenuCommand) {
				cmd := exec.Command(updateMenuCommand)
				err := cmd.Run()
				if err == nil {
					log.Println("Ran", updateMenuCommand, "command")
				} else {
					helpers.LogError("main: "+updateMenuCommand, err)
				}
			}

		}

		// Run update-desktop-database
		// "Build cache database of MIME types handled by desktop files."
		if helpers.IsCommandAvailable("update-desktop-database") {
			cmd := exec.Command("update-desktop-database", xdg.DataHome+"/applications/")
			err := cmd.Run()
			if err == nil {
				log.Println("Ran", "update-desktop-database "+xdg.DataHome+"/applications/")
			} else {
				helpers.LogError("main", err)
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
			watchedDirectories = helpers.AppendIfMissing(watchedDirectories, mount.MountPoint+"/Applications")
		}
	}

	log.Println("Registering AppImages in well-known locations and their subdirectories...")

	watchDirectoriesReally(watchedDirectories)

	helpers.DeleteDesktopFilesWithNonExistingTargets()
	// So this should also catch AppImages which were formerly hidden in some subdirectory
	// where the whole directory was deleted
}

func watchDirectoriesReally(watchedDirectories []string) {
	for _, v := range watchedDirectories {
		// TODO: Maybe we don't want to walk subdirectories?
		// filepath.Walk is handy but scans subfolders too, by default, which might not be what you want.
		// The Go stdlib also provides ioutil.ReadDir
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
		helpers.LogError("main: watchDirectoriesReally", err)
	}
}
