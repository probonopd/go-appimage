package main

import (
	"log"
	"os"

	//	"strings"

	"github.com/godbus/dbus"
)

func monitorUdisks(conn *dbus.Conn) {
	var rules = []string{
		// "path_namespace='/'", // Everything
		"interface='org.freedesktop.DBus'",
		"interface='org.gtk.Private.RemoteVolumeMonitor'", // FIXME: Do not rely on GTK stuff; is MountRemoved available in KDE too?
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
		log.Println("udisks:", v.Headers, v.Body)
		if v.Headers[3].Value() == "Hello" || v.Headers[3].Value() == "RemoveMatch" || v.Headers[3].Value() == "MountRemoved" {
			log.Println("udisks: XXXXX", v.Headers[3])
			watchDirectories()
		}
	}

}
