package main

import (
	"log"
	"os"

	"github.com/godbus/dbus"
)

/*

UDisks2VolumeMonitor seems to give us concise information about when a volume is mounted and unmounted:

me@host:~/go/src/github.com/probonopd/appimage/src/appimaged$ ./appimaged 2>&1  | grep XXX
2019/11/03 18:34:40 udisks: XXXXX map[1:@o "/org/gtk/Private/RemoteVolumeMonitor" 2:"org.gtk.Private.RemoteVolumeMonitor" 3:"MountAdded" 7:":1.51" 8:@g "ss(ssssssbsassa{sv})"] [org.gtk.vfs.UDisks2VolumeMonitor 0x55ec0dfab100 [0x55ec0dfab100 KINGSTON . GThemedIcon drive-harddisk-usb drive-harddisk drive . GThemedIcon drive-harddisk-usb-symbolic drive-harddisk-symbolic drive-symbolic drive-harddisk-usb drive-harddisk drive  file:///media/me/KINGSTON true 0x7f67c83b6320 [] gvfs.time_detected_usec.1572802480269453 map[]]]
2019/11/03 18:34:58 udisks: XXXXX map[1:@o "/org/gtk/Private/RemoteVolumeMonitor" 2:"org.gtk.Private.RemoteVolumeMonitor" 3:"MountRemoved" 7:":1.51" 8:@g "ss(ssssssbsassa{sv})"] [org.gtk.vfs.UDisks2VolumeMonitor 0x55ec0dfab100 [0x55ec0dfab100 KINGSTON . GThemedIcon drive-harddisk-usb drive-harddisk drive . GThemedIcon drive-harddisk-usb-symbolic drive-harddisk-symbolic drive-symbolic drive-harddisk-usb drive-harddisk drive  file:///media/me/KINGSTON true  [] gvfs.time_detected_usec.1572802480269453 map[]]]

Verified on
* Raspbian 10
* Xbuntu 18.04.2 LTS

Known NOT to work on
* neon-useredition-20190321-0530-amd64.iso

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

Wouldn't it make sense for XDG to standardize MountAdded, MountRemoved
dbus messages that would be broadcast everyone interested (so that one doesn't
need to "eavesdrop"), independent of whether the GNOME desktop is used?

*/

func monitorUdisks(conn *dbus.Conn) {

	// Check whether UDisks2VolumeMonitor is available, exit otherwise
	var s string
	bool satisfied := false
	e := conn.Object("org.gtk.vfs.UDisks2VolumeMonitor", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if e != nil {
		log.Println("Failed to introspect org.gtk.vfs.UDisks2VolumeMonitor", e)
	} else {
		satisfied = true
	}
	e = conn.Object("org.kde.Solid", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if e != nil {
		log.Println("Failed to introspect org.kde.Solid", e)
	} else {
		satisfied = true
	}
	
	if satisfied == false {
		os.Exit(1)
	}

	var rules = []string{
		// "path_namespace='/'", // Everything
		// "interface='org.gtk.Private.RemoteVolumeMonitor'",
		"member='MountAdded'",   // org.gtk.Private.RemoteVolumeMonitor
		"member='MountRemoved'", // org.gtk.Private.RemoteVolumeMonitor
		"member='setupDone'", // org.kde.Solid.Device
	}
	var flag uint = 0

	call := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, flag)
	if call.Err != nil {
		log.Println("Failed to become monitor:", call.Err)
		return
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)
	log.Println("monitor: Monitoring DBus session bus")

	for v := range c {
		log.Println("udisks: XXXXX", v.Headers, v.Body)
		// log.Println("udisks: XXXXX", v.Headers[3])
		watchDirectories()
	}

}
