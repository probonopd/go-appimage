package main

import (
	"log"
	"os"
	"time"

	"github.com/probonopd/go-appimage/internal/helpers"

	"github.com/godbus/dbus/v5"
)

/*

UDisks2VolumeMonitor seems to give us concise information about when a volume is mounted and unmounted:

me@host:~/go/src/github.com/probonopd/go-appimage/src/appimaged$ ./appimaged 2>&1  | grep XXX
2019/11/03 18:34:40 udisks: XXXXX map[1:@o "/org/gtk/Private/RemoteVolumeMonitor" 2:"org.gtk.Private.RemoteVolumeMonitor" 3:"MountAdded" 7:":1.51" 8:@g "ss(ssssssbsassa{sv})"] [org.gtk.vfs.UDisks2VolumeMonitor 0x55ec0dfab100 [0x55ec0dfab100 KINGSTON . GThemedIcon drive-harddisk-usb drive-harddisk drive . GThemedIcon drive-harddisk-usb-symbolic drive-harddisk-symbolic drive-symbolic drive-harddisk-usb drive-harddisk drive  file:///media/me/KINGSTON true 0x7f67c83b6320 [] gvfs.time_detected_usec.1572802480269453 map[]]]
2019/11/03 18:34:58 udisks: XXXXX map[1:@o "/org/gtk/Private/RemoteVolumeMonitor" 2:"org.gtk.Private.RemoteVolumeMonitor" 3:"MountRemoved" 7:":1.51" 8:@g "ss(ssssssbsassa{sv})"] [org.gtk.vfs.UDisks2VolumeMonitor 0x55ec0dfab100 [0x55ec0dfab100 KINGSTON . GThemedIcon drive-harddisk-usb drive-harddisk drive . GThemedIcon drive-harddisk-usb-symbolic drive-harddisk-symbolic drive-symbolic drive-harddisk-usb drive-harddisk drive  file:///media/me/KINGSTON true  [] gvfs.time_detected_usec.1572802480269453 map[]]]

Verified on
* Raspbian 10 (2019)
* Xbuntu 18.04.2 LTS (2018)
* Deepin 15.11 (2019)

Known NOT to work on
* neon-useredition-20190321-0530-amd64.iso (2019) - there we seemingly need to use Solid instead

However it seems to be related to the Virtual filesystem for the GNOME desktop
("gfvs", "GNOME VFS") rather than XDG unfortunately, and
sure enough it seems to be a Red Hat thing
https://github.com/gicmo/gvfs/blob/master/monitor/proxy/dbus-interfaces.xml
so it seems unlikely that it will be available everywhere. We need to investigate
this on other systems using e.g.,

dbus-monitor 2>&1 | grep -i mount

Which on KDE shows... nothing.
On KDE we get something else which mentions "freedesktop_2FUDisks2" though:

When a volume is mounted:
signal time=1572805120.557118 sender=:1.34 -> destination=(null destination) serial=227 path=/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1; interface=org.kde.Solid.Device; member=setupRequested
signal time=1572805120.612607 sender=:1.34 -> destination=(null destination) serial=228 path=/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1; interface=org.kde.Solid.Device; member=setupDone

When a volume is unmounted:
signal time=1572805151.390507 sender=:1.34 -> destination=(null destination) serial=230 path=/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1; interface=org.kde.Solid.Device; member=teardownRequested
signal time=1572805151.453440 sender=:1.34 -> destination=(null destination) serial=231 path=/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1; interface=org.kde.Solid.Device; member=teardownDone
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupDone'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownDone'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupDone'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownDone'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownRequested'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='teardownDone'"
   string "type='signal',path='/org/kde/solid/Device__2Forg_2Ffreedesktop_2FUDisks2_2Fblock_5Fdevices_2Fsdd1',interface='org.kde.Solid.Device',member='setupDone'"

Why such a storm of messages?

So it seems that despite mentions of "freedesktop" and "UDisks2", org.kde.Solid.Device
is responsible for these kinds of messages on KDE.

Solid is a device integration framework. It provides a way of querying and
interacting with hardware independently of the underlying operating system.
Apparently KDE Solid can use different backends, only one of which is UDisks2.
https://github.com/KDE/solid/tree/master/src/solid/devices/backends

Wouldn't it make sense for XDG to standardize MountAdded, MountRemoved
dbus messages that would be broadcast everyone interested (so that one doesn't
need to "eavesdrop"), independent of whether the GNOME or the KDE or another desktop is used?

*/

func monitorUdisks() {

	conn, err := dbus.SessionBusPrivate() // When using SessionBusPrivate(), need to follow with Auth(nil) and Hello()
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		helpers.PrintError("SessionBusPrivate", err)
		return
	}
	defer conn.Close()
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

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	if obj == nil {
		log.Println("ERROR: notification: obj is nil")
		os.Exit(1)
	}

	// Check whether UDisks2VolumeMonitor or org.kde.Solid is available, exit otherwise
	var s string
	satisfied := false
	e := conn.Object("org.gtk.vfs.UDisks2VolumeMonitor", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if e != nil {
		log.Println("Notice: Cannot introspect org.gtk.vfs.UDisks2VolumeMonitor on this system", e)
	} else {
		satisfied = true
	}
	retried := false
retry:
	// FIXME: For whatever reason this does NOT work with org.kde.Solid.Device
	// (which we actually care about), so we check the next best thing
	e = conn.Object("org.kde.Solid.PowerManagement", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if e != nil {
		if retried {
			log.Println("Notice: Cannot introspect org.kde.Solid on this system", e)
		} else {
			log.Println("org.kde.Solid.PowerManagement might not be started yet. Waiting a moment then retrying")
			time.Sleep(500 * time.Millisecond)
			retried = true
			goto retry
		}
	} else {
		satisfied = true
	}

	if !satisfied {
		sendErrorDesktopNotification("Cannot see volumes come and go", "Not implemented yet for this kind of system")
		log.Println("ERROR: Don't know how to get notified about mounted and unmounted devices on this system", e)
		log.Println("using dbus. Every system seems to do it differently.", e)
		// os.Exit(1)
		return
	}

	var rules = []string{
		// "path_namespace='/'", // Everything
		// "interface='org.gtk.Private.RemoteVolumeMonitor'",
		"member='MountAdded'",   // org.gtk.Private.RemoteVolumeMonitor
		"member='MountRemoved'", // org.gtk.Private.RemoteVolumeMonitor
		"member='setupDone'",    // org.kde.Solid.Device
		"member='teardownDone'", // org.kde.Solid.Device
	}
	var flag uint = 0

	call := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, flag)
	if call.Err != nil {
		log.Println("Failed to become monitor:", call.Err)
		os.Exit(1)
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)

	log.Println("monitor: Monitoring DBus session bus")

	for v := range c {
		log.Println("udisks headers:", v.Headers)
		log.Println("udisks body:", v.Body)
		// log.Println("udisks:", v.Headers[3])
		checkMounts()
	}
}

func checkMounts() {
	oldDirs := watchedDirectories[len(candidateDirectories):]
	newDirs := getMountDirectories()
old:
	for old := 0; old < len(oldDirs); old++ {
		for new := range newDirs {
			if oldDirs[old] == newDirs[new] {
				oldDirs = append(oldDirs[:old], oldDirs[old+1:]...)
				newDirs = append(newDirs[:new], newDirs[new+1:]...)
				old--
				continue old
			}
		}
	}
	for _, dir := range oldDirs {
		RemoveIntegrationsFromDir(dir)
		for i := range watchedDirectories {
			if watchedDirectories[i] == dir {
				watchedDirectories = append(watchedDirectories[:i], watchedDirectories[i+1:]...)
				break
			}
		}
		RemoveWatchDir(dir)
		log.Println("umounted: ", dir)
	}
	watchedDirectories = append(watchedDirectories, newDirs...)
	for _, dir := range newDirs {
		AddIntegrationsFromDir(dir)
		err := AddWatchDir(dir)
		helpers.PrintError("add watch", err)
		log.Println("now watching ", dir)
	}
}
