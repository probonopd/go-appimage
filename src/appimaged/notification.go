package main

// Notifies the user via desktop notifications
// by sending DBus messages.

import (
	"log"
	"os"

	"github.com/godbus/dbus"
)

func sendDesktopNotification(title string, body string) {

	log.Println("notification: Want to send", title, body)

	if conn == nil {
		log.Println("ERROR: notification: Could not get conn") // FIXME. Why don't I get conn here?
		return
	}

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	if obj == nil {
		log.Println("ERROR: notification: obj is nil")
		return
	}

	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{}, int32(5000))

	if call.Err != nil {
		log.Println("xxxxxxxxxxxxxxxxxxxx ERROR: notification:", call.Err) // Sometimes we get here: "read unix @->/run/user/999/bus: EOF"
		// From then on, we get "dbus: connection closed by user" for subsequent calls.
		// When this happens, then also the eavesdropping on UDisks2 etc. messages does no longer work.
		// Which means we are no longer functional, and have to exit.
		os.Exit(111)
		// FIXME: What to do instead? Any help appreciated.
	}

}
