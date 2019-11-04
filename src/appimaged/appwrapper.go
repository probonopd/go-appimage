// appwrapper executes applications and presents errors to the GUI as notifications
package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/godbus/dbus"
)

func appwrap() {

	if len(os.Args) < 3 {
		log.Println("Argument missing")
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)

	var out bytes.Buffer
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		log.Fatalf("cmd.Start: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				log.Printf("Exit Status: %d", status.ExitStatus())
				log.Println(out.String())

				summary := "Error"
				body := strings.TrimSpace(out.String())

				if strings.Contains(out.String(), "cannot open shared object file: No such file or directory") == true {
					parts := strings.Split(out.String(), ":")
					summary = "Error: Missing library " + strings.TrimSpace(parts[2])
					body = filepath.Base(os.Args[2]) + " could not be started because " + strings.TrimSpace(parts[2]) + " is missing"
				}
				sendErrorDesktopNotification(summary, body)

			}
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	}
}

// Send desktop notification. See
// https://developer.gnome.org/notification-spec/
func sendErrorDesktopNotification(title string, body string) {
	conn, err := dbus.SessionBus()
	defer conn.Close()
	if err != nil {
		log.Println(os.Stderr, "Failed to connect to session bus:", err)
		return
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{},
		int32(0)) // The timeout time in milliseconds at which the notification should automatically close.
	// If -1, the notification's expiration time is dependent on the notification server's settings,
	// and may vary for the type of notification.
	// If 0, the notification never expires.

	if call.Err != nil {
		panic(call.Err)
	}
}
