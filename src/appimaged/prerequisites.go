package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	helpers "github.com/probonopd/appimage/internal/helpers"
)

func checkPrerequisites() {

	ensureRunningFromLiveSystem()

	// Check if the tools that we need are available and warn if they are not
	// TODO: Elaborate checks whether the tools have the functionality we need (offset, ZISOFS)
	if helpers.IsCommandAvailable("unsquashfs") == false || helpers.IsCommandAvailable("bsdtar") == false {
		println("Required helper tools are missing.")
		println("Please make sure that recent versions of unsquashfs and bsdtar are on the $PATH")
		os.Exit(1)
	}

	// Check whether we have a sufficient version of unsquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("unsquashfs") == false {
		os.Exit(1)
	}

	// Stop any other AppImage system integration daemon
	// so that they won't interfere with each other
	stopSystemdService("appimagelauncherd")
	stopSystemdService("appimagelauncherfs")

	// Disable binfmt-misc of AppImageLauncher when we are NOT root? Argh!
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type1")
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type2")

	// Clean pre-existing desktop files and thumbnails
	// This is useful for debugging
	if *cleanPtr == true {
		files, err := filepath.Glob(filepath.Join(xdg.DataHome+"/applications/", "appimagekit_*"))
		helpers.LogError("main:", err)
		for _, file := range files {
			if *verbosePtr == true {
				log.Println("Deleting", file)
			}
			err := os.Remove(file)
			helpers.LogError("main:", err)
		}
		log.Println("Deleted", len(files), "desktop files from", xdg.DataHome+"/applications/; use -v to see details")

	}

	// E.g., on Xubuntu this directory is not there by default
	// but luckily it starts working right away without
	// the desktop needing to be restarted
	err := os.MkdirAll(xdg.DataHome+"/applications/", os.ModePerm)
	helpers.LogError("main:", err)
	err = os.MkdirAll(xdg.CacheHome+"/thumbnails/normal", os.ModePerm)
	helpers.LogError("main:", err)
	home, _ := os.UserHomeDir()
	err = os.MkdirAll(home+"/.cache/applications/", os.ModePerm)
	helpers.LogError("main:", err)

	// Create $HOME/.local/share/appimagekit/no_desktopintegration
	// so that AppImages know they should not do desktop integration themselves
	err = os.MkdirAll(xdg.DataHome+"/appimagekit/", os.ModePerm)
	helpers.LogError("main:", err)
	f, err := os.Create(xdg.DataHome + "/appimagekit/no_desktopintegration")
	helpers.LogError("main:", err)
	f.Close()
	helpers.LogError("main:", err)
}

func stopSystemdService(servicename string) {
	cmd := exec.Command("systemctl", "--user", "stop", servicename+".service")
	if err := cmd.Run(); err != nil {
		helpers.LogError(cmd.String(), err) // Needs Go 1.13
	} else {
		*cleanPtr = true // Clean up pre-existing desktop files from the other AppImage system integration daemon
	}
}

func exitIfBinfmtExists(path string) {
	cmd := exec.Command("/bin/sh", "-c", "echo -1 | sudo tee "+path)
	cmd.Run()
	if _, err := os.Stat(path); err == nil {
		log.Println("ERROR:", path, "exists. Please remove it by running")
		println("echo -1 | sudo tee", path)
		os.Exit(1)
	}
}

// ensureRunningFromLiveSystem checks if we are running on one of the supported Live systems
// and exits the process if we are not
func ensureRunningFromLiveSystem() {
	keywords := []string{"casper", "live", "Live", ".iso"}
	b, _ := ioutil.ReadFile("/proc/cmdline")
	str := string(b)
	found := false
	for _, k := range keywords {
		if strings.Contains(str, k) {
			found = true
		}
	}
	if found == false {
		println("Not running on one of the supported Live systems.")
		println("Grab a Ubuntu, Debian, Fedora, openSUSE, elementary OS, KDE neon,... Live ISO and try from there.")
		os.Exit(1)
	}
}
