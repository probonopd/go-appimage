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

However it seems to be related to the Virtual filesystem for the GNOME desktop
("gfvs", "GNOME VFS") rather than XDG unfortunately, and
sure enough it seems to be a Red Hat thing
https://github.com/gicmo/gvfs/blob/master/monitor/proxy/dbus-interfaces.xml
so it seems unlikely that it will be available everywhere. We need to investigate
this on other systems using e.g.,

dbus-monitor 2>&1 | grep -i mount

Wouldn't it make sense for XDG to standardize MountAdded, MountRemoved
dbus messages that would be broadcast everyone interested (so that one doesn't
need to "eavesdrop"), independent of whether the GNOME desktop is used?

*/

func monitorUdisks(conn *dbus.Conn) {

	// Check whether UDisks2VolumeMonitor is available, exit otherwise
	var s string
	e := conn.Object("org.gtk.vfs.UDisks2VolumeMonitor", "/").Call("org.freedesktop.DBus.Introspectable.Introspect", 0).Store(&s)
	if e != nil {
		log.Println(os.Stderr, "Failed to introspect org.gtk.vfs.UDisks2VolumeMonitor", e)
		os.Exit(1)
	}

	var rules = []string{
		// "path_namespace='/'", // Everything
		// "interface='org.gtk.Private.RemoteVolumeMonitor'",
		"member='MountAdded'",   // org.gtk.Private.RemoteVolumeMonitor
		"member='MountRemoved'", // org.gtk.Private.RemoteVolumeMonitor
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

	for v := range c {
		log.Println("udisks: XXXXX", v.Headers, v.Body)
		// log.Println("udisks: XXXXX", v.Headers[3])
		watchDirectories()
	}

}
