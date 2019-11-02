package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/adrg/xdg"
)

func checkPrerequisites() {
	// Check if the tools that we need are available and warn if they are not
	// TODO: Elaborate checks whether the tools have the functionality we need (offset, ZISOFS)
	checkToolAvailable("unsquashfs")
	checkToolAvailable("bsdtar")

	// Stop any other AppImage system integration daemon
	// so that they won't interfere with each other
	stopSystemdService("appimaged")
	stopSystemdService("appimagelauncherd")
	stopSystemdService("appimagelauncherfs")

	// TODO: How to disable binfmt-misc of AppImageLauncher when we are NOT root? Argh!
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type1")
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type2")

	// Clean pre-existing desktop files and thumbnails
	// This is useful for debugging
	if *cleanPtr == true {
		files, err := filepath.Glob(filepath.Join(xdg.DataHome+"/applications/", "appimagekit_*"))
		printError("main:", err)
		for _, file := range files {
			log.Println("Deleting", file)
			err := os.Remove(file)
			printError("main:", err)
		}

	}

	// E.g., on Xubuntu this directory is not there by default
	// but luckily it starts working right away without
	// the desktop needing to be restarted
	err := os.MkdirAll(xdg.DataHome+"/applications/", os.ModePerm)
	printError("main:", err)
	err = os.MkdirAll(xdg.CacheHome+"/thumbnails/normal", os.ModePerm)
	printError("main:", err)
	home, _ := os.UserHomeDir()
	err = os.MkdirAll(home+"/.cache/applications/", os.ModePerm)
	printError("main:", err)

	// Create $HOME/.local/share/appimagekit/no_desktopintegration
	// so that AppImages know they should not do desktop integration themselves
	err = os.MkdirAll(xdg.DataHome+"/appimagekit/", os.ModePerm)
	printError("main:", err)
	f, err := os.Create(xdg.DataHome + "/appimagekit/no_desktopintegration")
	printError("main:", err)
	f.Close()
	printError("main:", err)
}

// Print a warning if a tool is not there
func checkToolAvailable(toolname string) bool {
	if _, err := os.Stat(here() + "/" + toolname); os.IsNotExist(err) {
		log.Println("WARNING: bsdtar is missing in", here()+"/"+toolname+", functionality will be degraded")
		log.Println("You can get it from https://github.com/probonopd/static-tools/releases/tag/continuous")
		return false
	}
	return true
}

func stopSystemdService(servicename string) {
	cmd := exec.Command("systemctl", "--user", "stop", servicename+".service")
	if err := cmd.Run(); err != nil {
		printError(cmd.String(), err)
	} else {
		*cleanPtr = true // Clean up pre-existing desktop files from the other AppImage system integration daemon
	}
}

func exitIfBinfmtExists(path string) {
	if _, err := os.Stat(path); err == nil {
		log.Println("ERROR:", path, "exists. Please remove it by running")
		println("echo -1 | sudo tee", path)
		os.Exit(1)
	}
}
