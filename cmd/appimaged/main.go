package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/procfs"

	"github.com/probonopd/go-appimage/internal/helpers"
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

var updateChannel chan struct{} = make(chan struct{}, 10)

var thisai *AppImage // A reference to myself

var MQTTclient mqtt.Client

// This key in the desktop files written by us describes where the AppImage is in the filesystem.
// We need this because we rewrite Exec= to include things like wrap and Firejail
const ExecLocationKey = helpers.ExecLocationKey

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

var watchedDirectories []string

var home, _ = os.UserHomeDir()
var candidateDirectories = append(
	strings.Split(os.Getenv("PATH"), ":"),
	[]string{
		xdg.UserDirs.Download,
		xdg.UserDirs.Desktop,
		home + "/.local/bin",
		home + "/bin",
		home + "/Applications",
		"/opt",
		"/usr/local/bin",
	}...,
)

// Flags
var (
	verbose, quiet bool
	overwrite      *bool
	clean          *bool
	noZeroconf     *bool
	noMqtt         *bool
)

func usage() {
	if commit == "" {
		commit = "unsupported custom build"
	}
	fmt.Println()
	fmt.Println(filepath.Base(os.Args[0]) + " " + commit + "\n")
	fmt.Println()
	fmt.Println("Optional daemon that registers AppImages and integrates them with the system.")
	fmt.Println()
	fmt.Println("Sets the executable bit on AppImages, adds them to the system menu,")
	fmt.Println("and makes it possible to launch the most recent AppImage")
	fmt.Println("that is registered on the system for a given application.")
	fmt.Println()
	fmt.Println("If no commands are given, installs as a systemd service.")
	fmt.Println("If flags are specified, they will be copied to the systemd service.")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println()
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println()
	fmt.Println("\trun <updateinformation>:")
	fmt.Println("\t\tRun the most recent AppImage registered for the updateinformation provided")
	fmt.Println("\tstart <updateinformation>:")
	fmt.Println("\t\tStart the most recent AppImage registered for the updateinformation provided and exit immediately")
	fmt.Println("\tupdate <path to AppImage>:")
	fmt.Println("\t\tUpdate the AppImage using the most recent AppImageUpdate registered")
	fmt.Println("\twrap <path to executable>:")
	fmt.Println("\t\tExecute the exeutable and send desktop notifications for any errors")
	fmt.Println("\tservice")
	fmt.Println("\t\tStart appimaged daemon")
}

func main() {
	thisai, _ = NewAppImage(helpers.Args0())
	if commit == "" {
		commit = "unsupported custom build"
	}
	flag.Usage = usage
	verbosePtr := flag.Bool("v", false, "Print verbose log messages")
	quietPtr := flag.Bool("q", false, "Do not send desktop notifications")
	overwrite = flag.Bool("o", false, "Overwrite existing desktop integration files (slower)")
	clean = flag.Bool("c", false, "Clean pre-existing desktop files")
	noZeroconf = flag.Bool("nz", false, "Do not announce this service on the network using Zeroconf")
	noMqtt = flag.Bool("u", false, "Disable checking for AppImage updates (via MQTT)")
	flag.Parse()
	verbose = *verbosePtr
	quiet = *quietPtr
	if flag.NArg() == 0 {
		InstallSystemd()
	} else if flag.Arg(0) != "service" {
		HandleCommands()
	}

	// Always show version
	fmt.Println(filepath.Base(os.Args[0]), commit)

	checkPrerequisites()
	// fmt.Println("Setting as autostart...")
	// setMyselfAsAutostart()

	// Watch the filesystem for accesses using fanotify
	// FANotifyMonitor() // fanotifymonitor error: operation not permitted

	installFilemanagerContextMenus()

	// ptrue := true // Nasty trick from https://code-review.googlesource.com/c/gocloud/+/26730/3/bigquery/query.go
	// overwritePtr = &ptrue

	// Connect to MQTT server and subscribe to the topic for ourselves
	if !*noMqtt {
		if CheckIfConnectedToNetwork() {
			uri, err := url.Parse(helpers.MQTTServerURI)
			if err != nil {
				log.Fatal(err)
			}

			// go SubscribeMQTT(MQTTclient, "gh-releases-zsync|probonopd|merkaartor|continuous|Merkaartor-*-x86_64.AppImage.zsync")
			// go SubscribeMQTT(MQTTclient, "gh-releases-zsync|AppImage|AppImageKit|continuous|appimagetool-x86_64.AppImage.zsync")

			MQTTclient = connect("sub", uri)
			log.Println("MQTT client connected:", MQTTclient.IsConnected())
		}
	}

	// go monitorDbusSessionBus() // If used, then nothing else can use DBus anymore? FIXME #####################

	// SimpleNotify("Starting", helpers.Here(), 5000)

	log.Println("main: Running from", helpers.Here())
	log.Println("main: xdg.DataHome =", xdg.DataHome)

	helpers.DeleteDesktopFilesWithNonExistingTargets()

	log.Println("Overwrite:", *overwrite)
	log.Println("Clean:", *overwrite)

	// Disable desktop integration provided by scripts within AppImages
	// as per https://github.com/AppImage/AppImageSpec/blob/master/draft.md#desktop-integration
	err := os.Setenv("DESKTOPINTEGRATION", "go-appimaged")
	if err != nil {
		helpers.PrintError("main", err)
	}
	// TODO: Also react to network interfaces and network connections coming and going,
	// refer to the official NetworkManager dbus specification:
	// https://developer.gnome.org/NetworkManager/1.16/spec.html
	if !*noZeroconf {
		if CheckIfConnectedToNetwork() {
			go registerZeroconfService()
			go browseZeroconfServices()
		}
	}

	checkDirectories()
	for _, dir := range watchedDirectories {
		err = AddWatchDir(dir)
		if err != nil {
			log.Println("can't watch", dir, err)
		}
	}
	go StartWatch()

	// Try to register ourselves as a thumbnailer for AppImages, in the hope that
	// DBus notifications will be generated for AppImages as thumbnail-able files
	// FIXME: Currently getting: No such interface 'org.freedesktop.thumbnails' on object at path /org/freedesktop/thumbnails/Manager1
	// Maybe not needed? At least on Xubuntu it seems to work without this
	// but perhaps it is why KDE ignores our nice thumbnails

	// React to partitions being mounted and unmounted
	go monitorUdisks()

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

	var updateMenuTimer *time.Timer

	go func() {
		// Handles application menu updates.
		// At most, updates the application menu every second.
		for {
			<-updateChannel
			if updateMenuTimer == nil {
				updateMenuTimer = time.AfterFunc(time.Second, func() {
					updateMenu()
					updateMenuTimer = nil
				})
			}
		}
	}()

	<-quit

}

// checkMQTTConnected checks whether the MQTT client is
// still connected; try to reconnect if it is not.
// This is recommended by MQTT servers since they can go
// down for maintenance
func checkMQTTConnected(MQTTclient mqtt.Client) {
	if CheckIfConnectedToNetwork() {
		if !MQTTclient.IsConnected() {
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

// Periodically update the application menu so that the menu does not get rebuilt all the time
func updateMenu() error {
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
	return nil
}

func checkDirectories() {
	// Register AppImages from well-known locations
	// https://github.com/AppImage/appimaged#monitored-directories
	home, _ := os.UserHomeDir()
	err := os.MkdirAll(home+"/Applications", 0755)
	if err != nil {
		helpers.PrintError("main", err)
	}

	for _, dir := range candidateDirectories {
		if helpers.Exists(dir) {
			watchedDirectories = append(watchedDirectories, dir)
		}
	}
	watchedDirectories = append(watchedDirectories, getMountDirectories()...)

	for _, dir := range watchedDirectories {
		ents, err := os.ReadDir(dir)
		if err != nil {
			helpers.LogError("checkDirectories", err)
		}
		for _, fil := range ents {
			path := filepath.Join(dir, fil.Name())
			if IsPossibleAppImage(path) {
				log.Println("integrating", path)
				AddIntegration(path, false)
			}
		}
	}

	helpers.DeleteDesktopFilesWithNonExistingTargets()
	// So this should also catch AppImages which were formerly hidden in some subdirectory
	// where the whole directory was deleted
}

func getMountDirectories() (out []string) {
	out = make([]string, 0)

	mounts, _ := procfs.GetMounts()
	// FIXME: This breaks when the partition label has "-", see https://github.com/prometheus/procfs/issues/227

	for _, mount := range mounts {
		if verbose {
			log.Println("main: MountPoint", mount.MountPoint)
		}
		if !strings.HasPrefix(mount.MountPoint, "/sys") && // Is /dev needed for openSUSE Live?
			// strings.HasPrefix(mount.MountPoint, "/run") == false && // Manjaro mounts the device on which the Live ISO is in /run, so we cannot exclude that
			!strings.HasPrefix(mount.MountPoint, "/tmp") &&
			!strings.HasPrefix(mount.MountPoint, "/proc") {
			fmt.Println(mount.SuperOptions)
			if helpers.Exists(mount.MountPoint + "/Applications") {
				if _, ok := mount.SuperOptions["showexec"]; ok {
					go sendErrorDesktopNotification("UDisks showexec issue", "Applications cannot run from \n"+mount.MountPoint+". \nSee \nhttps://github.com/storaged-project/udisks/issues/707")
					printUdisksShowexecHint()
				} else {
					out = helpers.AppendIfMissing(out, mount.MountPoint+"/Applications")
				}
			}
		}
	}
	return
}
