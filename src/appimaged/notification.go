package main

// Notifies the user via desktop notifications
// by sending DBus messages.
// FIXME: Currently it does not crash but it does not work either
// when other things using DBus are already running (e.g, watching UDisks)

import (
	"log"
	"os"

	"github.com/probonopd/appimage/internal/helpers"

	"github.com/godbus/dbus"
)

func sendDesktopNotification(title string, body string, durationms int32) {

	conn, err := dbus.SessionBusPrivate() // When using SessionBusPrivate(), need to follow with Auth(nil) and Hello()
	defer conn.Close()
	if err != nil {
		helpers.PrintError("SessionBusPrivate", err)
		return
	}
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

	log.Println("notification: Want to send:", title, body)

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	if obj == nil {
		log.Println("ERROR: notification: obj is nil")
		return
	}

	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{}, int32(durationms))

	if call.Err != nil {
		log.Println("xxxxxxxxxxxxxxxxxxxx ERROR: notification:", call.Err)
		// Sometimes we get here: "read unix @->/run/user/999/bus: EOF"
		// means that we are not using PrivateConnection?
		os.Exit(111)
	}

}
