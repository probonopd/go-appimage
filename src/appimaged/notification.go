package main

// Notifies the user via desktop notifications
// by sending DBus messages.

import (
	"github.com/godbus/dbus"
)

func sendDesktopNotification(conn *dbus.Conn, title string, body string) {

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{}, int32(5000))
	if call.Err != nil {
		panic(call.Err)
	}
}
