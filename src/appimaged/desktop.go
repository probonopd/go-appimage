package main

// Handles reading, writing, installing, and verifying desktop files.
// Currently it is using using a static build of desktop-file-validate
// but eventually may be rewritten to do things natively in Go.

import (
	"log"
	"os/exec"

	"golang.org/x/sys/unix"

	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// If the AppImage is writeable (= the user can remove it), then add a "Move to Trash" action
	os.MkdirAll(xdg.DataHome+"/Trash/files/", 755)
	if isWritable(ai.path) {
		actions = append(actions, "Remove")
		cfg.Section("Desktop Action Remove").Key("Name").SetValue("Move to Trash")
		cfg.Section("Desktop Action Remove").Key("Exec").SetValue("mv '" + ai.path + "' " + xdg.DataHome + "/Trash/files/")
	}

	// TODO: Find usable (latest version of) AppImageUpdate and/or AppImageUpdater in a more fancy way
	if isCommandAvailable("AppImageUpdate") {
		actions = append(actions, "Update")
		cfg.Section("Desktop Action Update").Key("Name").SetValue("Update")
		cfg.Section("Desktop Action Update").Key("Exec").SetValue("AppImageUpdate '" + ai.path + "'")
	}

	as := ";"
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
