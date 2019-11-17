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
	"sync"
	"time"

	"github.com/adrg/xdg"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/probonopd/appimage/internal/helpers"
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

// The following are disabled for now, because the path to this program can
// change (e.g., when the user updates it). Lacking a system-wide Launch Services
// like way to figure out the path to this program, we overwrite for now.
// TODO: Instead of overwriting the desktop files and getting all
// information from AppImages (slow), we could just rewrite the path to this
// program in all desktop files. That should be much faster.
// var overwritePtr = flag.Bool("o", false, "Overwrite existing desktop integration")
var overwritePtr *bool
var cleanPtr = flag.Bool("c", false, "Clean pre-existing desktop files")

var quietPtr = flag.Bool("q", false, "Do not send desktop notifications")
var noZeroconfPtr = flag.Bool("nz", false, "Do not announce this service on the network using Zeroconf")

var toBeIntegratedOrUnintegrated []string

var thisai AppImage // A reference to myself

var MQTTclient mqtt.Client

// To keep track of what we already have subscribed. Something like this is needed in order
// not to be flooded with messages.
// If possible I would like to get rid of this slice,
// the mqtt library probably keeps track of this internally?
// Right now we never remove from this list for logical reasons
// (multiple AppImages may share the same updateinformation)...
// Checking whehter other AppImages are left is probably costly.
// So better find a way to get this information from the mqtt library.
var subscribedMQTTTopics []string

// This key in the desktop files written by us describes where the AppImage is in the filesystem.
// We need this because we rewrite Exec= to include things like wrap and Firejail
const ExecLocationKey = helpers.ExecLocationKey

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

func main() {

	// As quickly as possible go there if we are invoked from the command line with a command
	takeCareOfCommandlineCommands()

	var version string
	if commit != "" {
		version = commit
	} else {
		version = "unsupported custom build"
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" "+version+"\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Optional daemon that registers AppImages and integrates them with the system.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Sets the executable bit on AppImages, adds them to the system menu,\n")
		fmt.Fprintf(os.Stderr, "and makes it possible to launch the most recent AppImage\n that isregistered on the system for a given application.\n")
		fmt.Fprintf(os.Stderr, "\n")

		// FIXME: Someone please tell me how to do this using flag
		fmt.Fprintf(os.Stderr, "Commands: \n")
		fmt.Fprintf(os.Stderr, "run <updateinformation>:\n\tRun the most recent AppImage registered\n\tfor the updateinformation provided\n")
		fmt.Fprintf(os.Stderr, "start <updateinformation>:\n\tStart the most recent AppImage registered\n\tfor the updateinformation provided and exit immediately\n")
		fmt.Fprintf(os.Stderr, "update <path to AppImage>:\n\tUpdate the AppImage using the most recent\n\tAppImageUpdate registered\n")
		fmt.Fprintf(os.Stderr, "wrap <path to executable>:\n\tExecute the exeutable and send\n\tdesktop notifications for any errors\n")
		fmt.Fprintf(os.Stderr, "\n")

		flag.PrintDefaults()
	}
	flag.Parse()

	// Always show version
	fmt.Println(filepath.Base(os.Args[0]), version)

	checkPrerequisites()

	// Watch the filesystem for accesses using fanotify
	// FANotifyMonitor() // fanotifymonitor error: operation not permitted

	installFilemanagerContextMenus()

	ptrue := true // Nasty trick from https://code-review.googlesource.com/c/gocloud/+/26730/3/bigquery/query.go
	overwritePtr = &ptrue

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

	// go monitorDbusSessionBus() // If used, then nothing else can use DBus anymore? FIXME #####################

	// SimpleNotify("Starting", helpers.Here(), 5000)

	log.Println("main: Running from", helpers.Here())
	log.Println("main: xdg.DataHome =", xdg.DataHome)

	helpers.DeleteDesktopFilesWithNonExistingTargets()

	log.Println("Overwrite:", *overwritePtr)
	log.Println("Clean:", *overwritePtr)

	// Disable desktop integration provided by scripts within AppImages
	// as per https://github.com/AppImage/AppImageSpec/blob/master/draft.md#desktop-integration
	err := os.Setenv("DESKTOPINTEGRATION", "go-appimaged")
	if err != nil {
		helpers.PrintError("main", err)
	}
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

	// log.Println("Subscriptions:", subscribedMQTTTopics)

	// log.Println(watchedDirectories)
	// for _, w := range watchedDirectories {
	// 	log.Println(w.Path)
	// }

	// We want to know that all go routines have been completed,
	// and only then move in all desktop files at once
	// https://nathanleclaire.com/blog/2014/02/15/how-to-wait-for-all-goroutines-to-finish-executing-before-continuing/
	// For this we use "var wg sync.WaitGroup" before we start the go routines,
	// and "wg.Wait()" then waits until they have all done their job. Neat!
	var wg sync.WaitGroup

	// If we do everything in parallel, we get "too many files open" errors
	// Hence we limit the number of concurrent go routines
	// using a counting semaphore
	// https://stackoverflow.com/a/38825523
	sem := make(chan struct{}, 8) // Maximum number of concurrent go routines // ***

	for _, path := range toBeIntegratedOrUnintegrated {
		ai := NewAppImage(path)
		go ai.IntegrateOrUnintegrate()
		sem <- struct{}{}
		defer func() { <-sem }() // FIXME: Possible resource leak, 'defer' is called in a for loop - is this bad?
		defer wg.Done()
	}

	wg.Wait() // Wait until all go routines since "var wg sync.WaitGroup" have been completed
	close(sem)

	time.Sleep(time.Second * 1) // And wait one second longer to catch other AppImages that may have been come in the meantime

	toBeIntegratedOrUnintegrated = nil

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

	if len(files) != 0 {

		if *verbosePtr == true {
			log.Println("main: Moved", len(files), "desktop files to", xdg.DataHome+"/applications/")
		} else {
			log.Println("main: Moved", len(files), "desktop files to", xdg.DataHome+"/applications/; use -v to see details")
		}

		if len(files) == 1 {
			sendDesktopNotification("Added application", "", 5000)
		} else {
			sendDesktopNotification("Added "+strconv.Itoa(len(files))+" applications", "", 5000)
		}

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

	err := os.MkdirAll(home+"/Applications", 0755)
	if err != nil {
		helpers.PrintError("main", err)
	}
	var watchedDirectories []string

	candidateDirectories := []string{
		xdg.UserDirs.Download,
		xdg.UserDirs.Desktop,
		home + "/.local/bin",
		home + "/bin",
		home + "/Applications",
		home + "/opt",
		home + "/usr/local/bin",
	}

	for _, dir := range candidateDirectories {
		if Exists(dir) {
			watchedDirectories = append(watchedDirectories, dir)
		}
	}

	mounts, _ := procfs.GetMounts()
	// FIXME: This breaks when the partition label has "-", see https://github.com/prometheus/procfs/issues/227

	for _, mount := range mounts {
		if *verbosePtr == true {
			log.Println("main: MountPoint", mount.MountPoint)
		}
		if strings.HasPrefix(mount.MountPoint, "/sys") == false && // Is /dev needed for openSUSE Live?
			// strings.HasPrefix(mount.MountPoint, "/run") == false && // Manjaro mounts the device on which the Live ISO is in /run, so we cannot exclude that
			strings.HasPrefix(mount.MountPoint, "/tmp") == false &&
			strings.HasPrefix(mount.MountPoint, "/proc") == false {
			if Exists(mount.MountPoint + "/Applications") {
				watchedDirectories = helpers.AppendIfMissing(watchedDirectories, mount.MountPoint+"/Applications")
			}
		}
	}

	log.Println("Registering AppImages in", watchedDirectories)

	watchDirectoriesReally(watchedDirectories)

	helpers.DeleteDesktopFilesWithNonExistingTargets()
	// So this should also catch AppImages which were formerly hidden in some subdirectory
	// where the whole directory was deleted
}

func watchDirectoriesReally(watchedDirectories []string) {
	for _, v := range watchedDirectories {
		go inotifyWatch(v)
		// For now we don't walk subdirectories.
		// filepath.Walk scans subfolders too,
		// ioutil.ReadDir does not.
		infos, err := ioutil.ReadDir(v)
		if err != nil {
			helpers.PrintError("watchDirectoriesReally", err)
			return
		}
		for _, info := range infos {
			if err != nil {
				log.Printf("%v\n", err)
			} else if info.IsDir() == true {
				// go inotifyWatch(v + "/" + info.Name())
			} else if info.IsDir() == false {
				ai := NewAppImage(v + "/" + info.Name())
				if ai.imagetype > 0 {
					go ai.IntegrateOrUnintegrate()
				}
			}
		}
		helpers.LogError("main: watchDirectoriesReally", err)
	}
}
