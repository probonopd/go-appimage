// This is a very crude workaround since I cannot get desktop notifications
// to work in the same process as watching DBus. It should eventually go away.
// Any help appreciated. FIXME

package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/godbus/dbus"
)

// SimpleNotify sends a notification. It uses a very crude implementation
// by launching a new process
func SimpleNotify(title string, body string, timeout int) {
	if *quietPtr == false {
		exec.Command(os.Args[0], "notify", strconv.Itoa(timeout), title, body).Start()
	}
}

// JustNotify sends a desktop notification and then exits the process.
// It is intended to be used by calling this executable with the "justnotify"
// command as its first argument, the title as its second argument,
// and the body as its third argument.
// Trying to call this from within the main process will crash...
func JustNotify() {

	if len(os.Args) < 5 {
		log.Println("Argument missing")
		os.Exit(1)
	}
	title := os.Args[3]
	body := os.Args[4]
	timeout, _ := strconv.Atoi(os.Args[2])

	log.Println("----------------------------")
	log.Println("Notification:")
	log.Println(title)
	log.Println(timeout)

	conn, err := dbus.SessionBus()
	if err != nil {
		log.Println(os.Stderr, "Failed to connect to session bus:", err)
		return
	}
	if conn == nil {
		log.Println(os.Stderr, "Failed to get conn:", err)
		return
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{},
		int32(timeout)) // The timeout time in milliseconds at which the notification should automatically close.
	// If -1, the notification's expiration time is dependent on the notification server's settings,
	// and may vary for the type of notification.
	// If 0, the notification never expires.

	if call.Err != nil {
		log.Println("ERROR: justnotify:", call.Err)
	}
	os.Exit(0)
}
