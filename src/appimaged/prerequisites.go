package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/amenzhinsky/go-polkit"
	systemddbus "github.com/coreos/go-systemd/dbus"
	"github.com/probonopd/appimage/internal/helpers"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/process"
)

func checkPrerequisites() {

	// Ensure we are running on a Live system
	// Maybe in the distant future this may go away but for now we want everyone
	// who tests or works on this to help making it a 1st class experience on Live systems
	// becase we deeply care about those. Yes, AppImage is "opinionated".
	// Once the experience on Live systems is as intended, we may lift this restriction.
	// We want to prevent people from working on this code without caring about
	// Live systems.
	ensureRunningFromLiveSystem()

	// Check for needed files on $PATH
	helpers.AddHereToPath() // Add the location of the executable to the $PATH
	tools := []string{"bsdtar", "unsquashfs", "desktop-file-validate"}
	err := helpers.CheckForNeededTools(tools)
	if err != nil {
		os.Exit(1)
	}

	// Poor man's singleton
	// Ensure that no other processes with the same name are already runing under the same user
	TerminateOtherInstances()

	// We really don't want users to run this in any other way than from an AppImage
	// because it only creates support issues and we can't update this AppImage
	// using our own dogfood
	// The ONLY exception is developers that know what they are doing
	_, aiEnvIsThere := os.LookupEnv("APPIMAGE")
	_, gcEnvIsThere := os.LookupEnv("GOCACHE")
	if aiEnvIsThere == false {
		// log.Println(os.Environ())
		log.Println("Running from AppImage type", thisai.imagetype)
		if gcEnvIsThere == false {
			log.Println("Not running from within an AppImage, exiting")
			os.Exit(1)
		} else {
			// Note that this exception is for use during develpment of this tool only and may go away at any time.
			sendDesktopNotification("Not running from an AppImage", "This is discouraged because some functionality may not be available", 5000)
		}
	}

	// Check whether we have a sufficient version of unsquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("unsquashfs") == false {
		os.Exit(1)
	}

	// Stop any other AppImage system integration daemon
	// so that they won't interfere with each other
	if checkIfSystemdServiceRunning([]string{"appimagelauncher*"}) == true {
		log.Println("Trying to stop interfering AppImage system integration daemons")
		stopSystemdService("appimagelauncherd")
		stopSystemdService("appimagelauncherfs")
	}

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
		if *verbosePtr == true {
			log.Println("Deleted", len(files), "desktop files from", xdg.DataHome+"/applications/")
		} else {
			log.Println("Deleted", len(files), "desktop files from", xdg.DataHome+"/applications/; use -v to see details")
		}
	}

	// E.g., on Xubuntu this directory is not there by default
	// but luckily it starts working right away without
	// the desktop needing to be restarted
	err = os.MkdirAll(xdg.DataHome+"/applications/", os.ModePerm)
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

func checkIfSystemdServiceRunning(servicenames []string) bool {

	conn, err := systemddbus.NewUserConnection()
	defer conn.Close()
	helpers.PrintError("pre: checkIfSystemdServiceRunning", err)
	if err != nil {
		return false
	}
	if conn == nil {
		log.Println("ERROR: checkIfSystemdServiceRunning: Could not get conn")
		return false
	}

	units, err := conn.ListUnitsByPatterns([]string{}, servicenames)
	helpers.PrintError("pre: checkIfSystemdServiceRunning", err)
	if err != nil {
		return false
	}

	for _, unit := range units {
		log.Println(unit.Name, unit.ActiveState)
	}

	if len(units) > 0 {
		return true
	} else {
		return false
	}

}

// stopSystemdService stops a SystemKit service with servicename.
// It asks the user for permission using polkit if needed.
func stopSystemdService(servicename string) {
	// Get permission from polkit to manage systemd units
	// Why do we need permission to run systemctl --user on e.g., Clear Linux OS?
	// After all, it is --user...
	authority, err := polkit.NewAuthority()
	if err != nil {
		helpers.PrintError("polkit", err)
	}
	action := "org.freedesktop.systemd1.manage-units"
	result, err := authority.CheckAuthorization(action, nil, polkit.CheckAuthorizationAllowUserInteraction, "")
	if err != nil {
		// helpers.PrintError("stopSystemdService", err)
		// This is not really an error; hence intentionally do nothing here
	}

	log.Printf("polkit: Is authorized: %t %s\n", result.IsAuthorized, action)

	// Stop systemd service

	// cmd := exec.Command("systemctl", "--user", "stop", servicename+".service")
	// if err := cmd.Run(); err != nil {
	// 	helpers.LogError(cmd.String(), err) // Needs Go 1.13
	// } else {
	// 	*cleanPtr = true // Clean up pre-existing desktop files from the other AppImage system integration daemon
	// }

	conn, err := systemddbus.NewUserConnection()
	defer conn.Close()
	// helpers.PrintError("pre: checkIfSystemdServiceRunning", err)
	if err != nil {
		return
	}
	if conn == nil {
		log.Println("ERROR: stopSystemdService: Could not get conn")
		return
	}

	reschan := make(chan string) // needed to wait for StopUnit job to complete
	_, err = conn.StopUnit(servicename, "replace", reschan)
	helpers.PrintError("pre: StopUnit", err)
	if err != nil {
		return
	}
	<-reschan // wait for StopUnit job to complete
}

func exitIfBinfmtExists(path string) {
	cmd := exec.Command("/bin/sh", "-c", "echo -1 | sudo tee "+path)
	err := cmd.Run()
	if err != nil {
		helpers.PrintError("prerequisites: exitIfBinfmtExists", err)
	}
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
	_, gcEnvIsThere := os.LookupEnv("GOCACHE")
	if found == false && gcEnvIsThere == false {
		sendDesktopNotification("Not running on one of the supported Live systems", "Grab a Ubuntu, Debian, Deepin, Fedora, openSUSE, elementary OS, KDE neon,... Live ISO and try from there.", -1)
		os.Exit(1)
	}
}

// TerminateOtherInstances sends the SIGTERM signal to all other processes of the same user
// that have the same process name as the current one in their name
func TerminateOtherInstances() {
	infoStat, _ := host.Info()
	fmt.Printf("Total processes: %d\n", infoStat.Procs)

	miscStat, _ := load.Misc()
	fmt.Printf("Running processes: %d\n", miscStat.ProcsRunning)

	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	myself, _ := os.Readlink("/proc/self/exe")
	fmt.Println("Process name based on /proc/self/exe:", filepath.Base(myself))
	fmt.Println("Terminating other running processes with that name...")

	var pids []int32
	procs, _ := process.Processes()
	for _, p := range procs {
		cmdline, _ := p.Cmdline()
		// FIXME: We must take care not to terminating the AppImage runtime that we are running ourselves from;
		// we do this by not terminating anything with ".AppImage" in its filename. This logic should be changed to
		// exclusively use PIDs instead, so that it works independent of file suffixes
		if strings.Contains(cmdline, filepath.Base(myself)) == true && strings.Contains(cmdline, "wrap") == false && strings.Contains(cmdline, ".AppImage") == false {
			procusername, err := p.Username()
			if err != nil {
				panic(err)
			}

			if user.Username == procusername && p.Pid != int32(os.Getpid()) {
				pids = append(pids, p.Pid)
				for _, pid := range pids {
					fmt.Println("Sending SIGTERM to", pid)
					syscall.Kill(int(pid), syscall.SIGTERM)
				}
			}
		}
	}
}
