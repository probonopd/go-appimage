package main

// Handles reading, writing, installing, and verifying desktop files.
// Currently it is using using a static build of desktop-file-validate
// but eventually may be rewritten to do things natively in Go.

import (
	"log"
	"os/exec"

	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/adrg/xdg"
	"gopkg.in/ini.v1"
)

// Write desktop file for a given AppImage to a temporary location.
// Call this with "go" because we have intentional delay in here (we are waiting for
// external thumbnailers to complete), which means it does not return
// for a while
func writeDesktopFile(ai AppImage) {

	filename := "appimagekit_" + ai.md5 + ".desktop"

	// log.Println(md5s)
	// XDG directories
	// log.Println(xdg.DataHome)
	// log.Println(xdg.DataDirs)
	// log.Println(xdg.ConfigHome)
	// log.Println(xdg.ConfigDirs)
	desktopcachedir := xdg.CacheHome + "/applications/" // FIXME: Do not hardcode here and in other places

	err := os.MkdirAll(desktopcachedir, os.ModePerm)
	if err != nil {
		log.Printf("desktop: %v", err)
	}
	// log.Println(xdg.RuntimeDir)

	// TODO: Instead of starting with an empty file, start with reading the original one
	// cfg, err := ini.Load("my.ini")
	// if err != nil {
	// 	fmt.Printf("Fail to read file: %v", err)
	// 	os.Exit(1)
	// }

	cfg := ini.Empty()

	// FIXME: KDE seems to have a problem when the AppImage is on a partition of which the disklabel contains "_"?
	// Then the desktop file won't run the application
	cfg.Section("Desktop Entry").Key("Exec").SetValue(ai.path) // KDE does not accept, even if desktop-file-validate allows ("\"" + ai.path + "\"")
	// cfg.Section("Desktop Entry").Key("TryExec").SetValue(ai.path) // KDE does not accept, even if desktop-file-validate allows ("\"" + ai.path + "\"")
	cfg.Section("Desktop Entry").Key("Type").SetValue("Application")
	// Construct the Name entry based on the actual filename
	// so that renaming the file in the file manager results in a changed name in the menu
	niceName := filepath.Base(ai.path)
	niceName = strings.Replace(niceName, ".AppImage", "", -1)
	niceName = strings.Replace(niceName, ".appimage", "", -1)
	niceName = strings.Replace(niceName, "-x86_64", "", -1)
	niceName = strings.Replace(niceName, "-i386", "", -1)
	niceName = strings.Replace(niceName, "-i686", "", -1)
	niceName = strings.Replace(niceName, "-", " ", -1)
	niceName = strings.Replace(niceName, "_", " ", -1)
	cfg.Section("Desktop Entry").Key("Name").SetValue(niceName)
	home, _ := os.UserHomeDir()
	thumbnail := home + "/.thumbnails/normal/" + ai.md5 + ".png"
	// FIXME: If the thumbnail is not generated here but by another external thumbnailer, it may not be fast enough
	time.Sleep(1 * time.Second)
	// For icons, use absolute paths. This way icons start working
	// without having to restart the desktop, and possibly
	// we can even get around messing around with the XDG icon spec
	// that expects different sizes of icons in different directories

	cfg.Section("Desktop Entry").Key("Icon").SetValue(thumbnail)
	/*
		if _, err := os.Stat(thumbnail); err == nil {
			// Thumbnail exists, then we use it as the Icon in the desktop file
			// TODO: Maybe we should assume the icon exists; and only thereafter "go extract it" for performance
			// so that we get the menu entries even if the icons have not been extracted yet
			cfg.Section("Desktop Entry").Key("Icon").SetValue(thumbnail)
		} else if os.IsNotExist(err) {
			// Thumbnail  does *not* exist, then we use a default application icon (better than nothing)
			cfg.Section("Desktop Entry").Key("Icon").SetValue("application-default-icon") // Use this if no other one is set or it is not found
			// TODO: Move away from here. Make one image struct regardless of type?

		}
	*/
	cfg.Section("Desktop Entry").Key("Comment").SetValue(ai.path)
	cfg.Section("Desktop Entry").Key("X-AppImage-Identifier").SetValue(ai.md5)

	// Actions

	var actions []string

	// Add "Move to Trash" action
	// if the AppImage is writeable (= the user can remove it)
	//
	// FIXME: The current implementation is desktop specfific and breaks
	// if the user uses the same home directory with multiple desktops.
	// Why isn't there a XDG standard tool or dbus call to move files to the Trash?
	// According to http://xahlee.info/linux/linux_trash_location.html:
	// Where is the trash directory?
	// ~/.local/share/Trash/ → on your local file system.
	// /root/.local/share/Trash/ → if you are root, on your local file system.
	// /media/PENDRIVE/.Trash-1000/ → on a USB drive.

	if isWritable(ai.path) {
		actions = append(actions, "Remove")
		cfg.Section("Desktop Action Remove").Key("Name").SetValue("Move to Trash")
		if isCommandAvailable("gio") {
			// A command line tool to move files to the Trash. However, GNOME-specific
			cfg.Section("Desktop Action Remove").Key("Exec").SetValue("gio trash '" + ai.path + "'")
		} else if isCommandAvailable("kioclient") {
			// Of course KDE has its own facility for doing the exact same thing
			cfg.Section("Desktop Action Remove").Key("Exec").SetValue("kioclient move '" + ai.path + "' trash:/")
		}
	}

	// Add "Update" action
	// TODO: Find usable (latest version of) AppImageUpdate and/or AppImageUpdater in a more fancy way
	if isCommandAvailable("AppImageUpdate") {
		actions = append(actions, "Update")
		cfg.Section("Desktop Action Update").Key("Name").SetValue("Update")
		cfg.Section("Desktop Action Update").Key("Exec").SetValue("AppImageUpdate '" + ai.path + "'")
	}

	// Add "Open Containing Folder" action
	if isCommandAvailable("xdg-open") {
		actions = append(actions, "Show")
		cfg.Section("Desktop Action Show").Key("Name").SetValue("Open Containing Folder")
		cfg.Section("Desktop Action Show").Key("Exec").SetValue("xdg-open '" + filepath.Clean(ai.path+"/../") + "'")
	}

	/*
	   # The simplest and most straightforward way to get the most recent version
	   # of Firejail running on a less than recent OS; don't do this at home kids
	   FILE=$(wget -q "http://dl-cdn.alpinelinux.org/alpine/edge/main/x86_64/" -O - | grep musl-1 | head -n 1 | cut -d '"' -f 2)
	   wget -c "http://dl-cdn.alpinelinux.org/alpine/edge/main/x86_64/$FILE"
	   FILE=$(wget -q "http://dl-cdn.alpinelinux.org/alpine/edge/community/x86_64/" -O - | grep firejail-0 | head -n 1 | cut -d '"' -f 2)
	   wget -c "http://dl-cdn.alpinelinux.org/alpine/edge/community/x86_64/$FILE"
	   sudo tar xf musl-*.apk -C / 2>/dev/null
	   sudo tar xf firejail-*.apk -C / 2>/dev/null
	   sudo chown root:root /usr/bin/firejail ; sudo chmod u+s /usr/bin/firejail # suid
	*/

	// Add "Run in Firejail" action
	if isCommandAvailable("firejail") {
		actions = append(actions, "Firejail")
		cfg.Section("Desktop Action Firejail").Key("Name").SetValue("Run in Firejail")
		cfg.Section("Desktop Action Firejail").Key("Exec").SetValue("firejail --env=DESKTOPINTEGRATION=appimaged --noprofile --appimage '" + ai.path + "'")

		actions = append(actions, "FirejailNoNetwork")
		cfg.Section("Desktop Action FirejailNoNetwork").Key("Name").SetValue("Run in Firejail Without Network Access")
		cfg.Section("Desktop Action FirejailNoNetwork").Key("Exec").SetValue("firejail --env=DESKTOPINTEGRATION=appimaged --noprofile --net=none --appimage '" + ai.path + "'")

		actions = append(actions, "FirejailPrivate")
		cfg.Section("Desktop Action FirejailPrivate").Key("Name").SetValue("Run in Private Firejail Sandbox")
		cfg.Section("Desktop Action FirejailPrivate").Key("Exec").SetValue("firejail --env=DESKTOPINTEGRATION=appimaged --noprofile --private --appimage '" + ai.path + "'")
	}

	as := ""
	for _, action := range actions {
		as = as + action + ";"
	}
	cfg.Section("Desktop Entry").Key("Actions").SetValue(as)

	log.Println("desktop: Saving to", desktopcachedir+filename)
	err = cfg.SaveTo(desktopcachedir + filename)
	if err != nil {
		log.Printf("Fail to write file: %v", err)
	}
	err = os.Chmod(desktopcachedir+filename, 0755)
	if err != nil {
		log.Println(err)
	}
}

func checkIfExecFileExists(desktopfilepath string) bool {
	_, err := os.Stat(desktopfilepath)
	if os.IsNotExist(err) {
		return false
	}
	cfg, e := ini.Load(desktopfilepath)
	printError("desktop", e)
	dst := cfg.Section("Desktop Entry").Key("Exec").String()

	_, err = os.Stat(dst)
	if os.IsNotExist(err) {
		log.Println(dst, "does not exist, it is mentioned in", desktopfilepath)
		return false
	}
	return true
}

func deleteDesktopFilesWithNonExistingTargets() {
	files, e := ioutil.ReadDir(xdg.DataHome + "/applications/")
	printError("desktop", e)
	if e != nil {
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") && strings.HasPrefix(file.Name(), "appimagekit_") {
			exists := checkIfExecFileExists(xdg.DataHome + "/applications/" + file.Name())
			if exists == false {
				log.Println("Deleting", xdg.DataHome+"/applications/"+file.Name())
				e = os.Remove(xdg.DataHome + "/applications/" + file.Name())
				printError("desktop", e)
			}
		}
	}

}

// Return true if a path to a file is writable
func isWritable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}

// Return true if a file is on the $PATH
func isCommandAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
