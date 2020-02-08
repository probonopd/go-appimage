package main

// Notifies the user via desktop notifications
// by sending DBus messages.
// FIXME: Currently it does not crash but it does not work either
// when other things using DBus are already running (e.g, watching UDisks)

import (
	"log"
	"os"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus"
	"github.com/probonopd/go-appimage/internal/helpers"
)

// sendUpdateDesktopNotification sends a desktop notification for an update.
// Use this with "go" prefixed to it so that it runs in the background, because it waits
// until the user clicks on "Update" or the timeout occurs
func sendUpdateDesktopNotification(ai AppImage, version string, changelogUrl string) {

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

	// Create a Notification to send
	iconName := "software-update-available"
	n := notify.Notification{
		AppName:       ai.niceName,
		ReplacesID:    uint32(0),
		AppIcon:       iconName,
		Summary:       "Update available",
		Body:          ai.niceName + " can be updated to version " + version + ". \n<a href='" + changelogUrl + "'>View Changelog</a>",
		Actions:       []string{"update", "Update", "changelog", "View Changelog"}, // tuples of (action_key, label)
		Hints:         map[string]dbus.Variant{},
		ExpireTimeout: int32(120000),
	}

	// List server capabilities
	caps, err := notify.GetCapabilities(conn)
	if err != nil {
		log.Printf("error fetching capabilities: %v", err)
	}
	for x := range caps {
		log.Printf("Registered capability: %v\n", caps[x])
	}

	// TODO: Only send fancy notification if server has the capabilities for it,
	// otherwise fall back to simple sendDesktopNotification()

	info, err := notify.GetServerInformation(conn)
	if err != nil {
		log.Printf("error getting server information: %v", err)
	}
	log.Printf("Name:    %v\n", info.Name)
	log.Printf("Vendor:  %v\n", info.Vendor)
	log.Printf("Version: %v\n", info.Version)
	log.Printf("Spec:    %v\n", info.SpecVersion)

	// Notifier interface with event delivery
	notifier, err := notify.New(conn)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer notifier.Close()

	id, err := notifier.SendNotification(n)
	if err != nil {
		log.Printf("error sending notification: %v", err)
	}
	log.Printf("sent notification id: %v", id)

	// Listen for actions invoked
	actions := notifier.ActionInvoked()
	go func() {
		action := <-actions
		if action != nil { // Without this if we get a crash if user just closes the notification w/o an action
			log.Printf("ActionInvoked: %v Key: %v", action.ID, action.ActionKey)
			if action.ActionKey == "update" {
				log.Println("TODO: Update to be implemented here")
				runUpdate(ai.path)
			}
		}
	}()

	closer := <-notifier.NotificationClosed() // Without this it doesn't wait for a user reaction

	/*
		FIXME: This seems to be triggered on all, not only on the matching ID
		So when the user closes one notification, the others don't function anymore
		is this a bug in the library?

		2020/02/08 09:14:49 sent notification id: 42
		(...)
		2020/02/08 09:14:49 sent notification id: 43
		(user dismisses ONE of them, we get:)
		2020/02/08 09:15:15 NotificationClosed: 43 Reason: DismissedByUser
		2020/02/08 09:15:15 its all over, go home
		2020/02/08 09:15:15 NotificationClosed: 43 Reason: DismissedByUser
		2020/02/08 09:15:15 its all over, go home
		(the buttons in notification 42 are now without function)
	*/

	if closer != nil {
		log.Printf("NotificationClosed: %v Reason: %v", closer.ID, closer.Reason)
	}

}

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

	log.Println("Desktop notification: ", title, body)

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	if obj == nil {
		log.Println("ERROR: notification: obj is nil")
		return
	}

	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, body, []string{},
		map[string]dbus.Variant{}, durationms)

	if call.Err != nil {
		log.Println("xxxxxxxxxxxxxxxxxxxx ERROR: notification:", call.Err)
		// Sometimes we get here: "read unix @->/run/user/999/bus: EOF"
		// means that we are not using PrivateConnection?
		os.Exit(111)
	}

}
