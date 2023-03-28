package main

import (
	"context"
	"fmt"

	"github.com/adrg/xdg"

	//	"github.com/amenzhinsky/go-polkit"

	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"strings"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/shirou/gopsutil/process"
)

func checkPrerequisites() {
	// Check for needed files on $PATH
	helpers.AddHereToPath() // Add the location of the executable to the $PATH
	// Add the watched directories to the $PATH
	helpers.AddDirsToPath(watchedDirectories)

	err := helpers.CheckForNeededTools("bsdtar", "desktop-file-validate")
	if err != nil {
		os.Exit(1)
	}

	// Poor man's singleton
	// Ensure that no other processes with the same name are already running under the same user
	TerminateOtherInstances()

	// We really don't want users to run this in any other way than from an AppImage
	// because it only creates support issues and we can't update this AppImage
	// using our own dogfood
	// The ONLY exception is developers that know what they are doing
	_, aiEnvIsThere := os.LookupEnv("APPIMAGE")
	_, gcEnvIsThere := os.LookupEnv("GOCACHE")
	if !aiEnvIsThere {
		// log.Println(os.Environ())
		log.Println("Running from AppImage type", thisai.Type())
		if !gcEnvIsThere {
			log.Println("Not running from within an AppImage, exiting")
			os.Exit(1)
		} else {
			// Note that this exception is for use during development of this tool only and may go away at any time.
			sendDesktopNotification("Not running from an AppImage", "This is discouraged because some functionality may not be available", 5000)
		}
	}

	// Check whether we have a sufficient version of unsquashfs for -offset
	if !helpers.CheckIfSquashfsVersionSufficient("unsquashfs") {
		os.Exit(1)
	}

	// Stop any other AppImage system integration daemon
	// so that they won't interfere with each other
	if checkIfSystemdServiceRunning([]string{"appimagelauncher*"}) {
		sendErrorDesktopNotification("Other AppImage integration daemon detected", "Please uninstall appimagelauncher first, then try again")
		os.Exit(1)
		// log.Println("Trying to stop interfering AppImage system integration daemons")
		// stopSystemdService("appimagelauncherd")
		// stopSystemdService("appimagelauncherfs")
	}

	// Disable binfmt-misc of AppImageLauncher when we are NOT root? Argh!
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type1")
	exitIfBinfmtExists("/proc/sys/fs/binfmt_misc/appimage-type2")

	// Clean pre-existing desktop files and thumbnails
	// This is useful for debugging
	if *clean {
		var files []string
		files, err = filepath.Glob(filepath.Join(xdg.DataHome, "applications", "appimagekit_*"))
		helpers.LogError("main:", err)
		for _, file := range files {
			if verbose {
				log.Println("Deleting", file)
			}
			err = os.Remove(file)
			helpers.LogError("main:", err)
		}
		if verbose {
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
	err = os.MkdirAll(ThumbnailsDirNormal, os.ModePerm)
	helpers.LogError("main:", err)
	home, _ := os.UserHomeDir()
	err = os.MkdirAll(home+"/.cache/applications/", os.ModePerm)
	helpers.LogError("main:", err)

	// Some systems may expect thumbnails in another (old?) location. Use that old location if it exists and the new location does not exist
	// TODO: Find a more robust mechanism
	if !helpers.Exists(ThumbnailsDirNormal) && helpers.Exists(home+"/.thumbnails/normal/") {
		log.Println("Using", ThumbnailsDirNormal, "as the location for thumbnails")
		ThumbnailsDirNormal = home + "/.thumbnails/normal/"
	}

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

	cont := context.Background()

	conn, err := systemddbus.NewUserConnectionContext(cont)
	helpers.PrintError("pre: checkIfSystemdServiceRunning", err)
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		return false
	}
	defer conn.Close()
	if conn == nil {
		log.Println("ERROR: checkIfSystemdServiceRunning: Could not get conn")
		return false
	}

	units, err := conn.ListUnitsByPatternsContext(cont, []string{}, servicenames)
	helpers.PrintError("pre: checkIfSystemdServiceRunning", err)
	if err != nil {
		return false
	}

	for _, unit := range units {
		log.Println(unit.Name, unit.ActiveState)
	}

	return len(units) > 0
}

/*
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
*/

func exitIfBinfmtExists(path string) {
	if _, err := os.Stat(path); err == nil {
		//If path exists, try to get sudo and remove it.
		cmd := exec.Command("/bin/sh", "-c", "echo -1 | sudo tee "+path)
		cmd.Run()
		if _, err := os.Stat(path); err == nil {
			//It still exists, so we panic and quit.
			log.Println("ERROR:", path, "exists. Please remove it by running")
			println("echo -1 | sudo tee", path)
			os.Exit(1)
		}
	}
}

// TerminateOtherInstances sends the SIGTERM signal to all other processes of the same user
// that have the same process name as the current one in their name
// FIXME: Since this is not working properly yet, we are just printing for now but not acting
func TerminateOtherInstances() {
	user, err := user.Current()
	if err != nil {
		helpers.LogError("term other instances", err)
		return
	}
	myself, _ := os.Readlink("/proc/self/exe")
	fmt.Println("This process based on /proc/self/exe:", myself)
	fmt.Println("Terminating other running processes with that name...")

	var pids []int32
	appImageEnv, _ := os.LookupEnv("APPIMAGE")
	procs, _ := process.Processes()
	for _, p := range procs {
		cmdline, _ := p.Cmdline()
		// Do not terminate instances that were called with a verb, and our own AppImage
		if strings.Contains(cmdline, filepath.Base(myself)) &&
			!strings.Contains(cmdline, "wrap") &&
			!strings.Contains(cmdline, "run") &&
			!strings.Contains(cmdline, appImageEnv) &&
			!strings.Contains(cmdline, myself) {
			procusername, err := p.Username()
			if err != nil {
				panic(err)
			}
			if user.Username == procusername && p.Pid != int32(os.Getpid()) {
				pids = append(pids, p.Pid)
			}
		}
	}
	for _, pid := range pids {
		fmt.Println("In the future, would send SIGTERM to", pid)
		/*
			err = syscall.Kill(int(pid), syscall.SIGTERM)
			if err != nil {
				panic(err)
			}
		*/
	}
}

func printUdisksShowexecHint() {
	fmt.Println(`You could run the following as a workaround. USE AT YOUR OWN RISK:
+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
sudo su
systemctl stop udisks2
if [ -e /usr/lib/udisks/udisks-daemon ] ; then
  sed -i -e 's|showexec|\x00\x00\x00\x00\x00\x00\x00\x00|g' /usr/lib/udisks/udisks-daemon
fi
if [ -e /usr/lib/udisks2/udisksd ] ; then
  sed -i -e 's|showexec|\x00\x00\x00\x00\x00\x00\x00\x00|g' /usr/lib/udisks2/udisksd
fi
if [ -e /usr/libexec/udisks2/udisksd ] ; then
  sed -i -e 's|showexec|\x00\x00\x00\x00\x00\x00\x00\x00|g' /usr/libexec/udisks2/udisksd
fi
systemctl restart udisks2
++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++`)
}

// setupToRunThroughSystemd checks if this process has been launched through
// systemd on a systemd system and takes appropriate measures if it has not,
// either because systemd was not yet set up to launch it, or because
// another (newer?) version has been launched manually by the user outside
// of systemd
func InstallSystemd() {

	// When this process is being launched, then check whether we have been
	// launched by systemd. If the system is using systemd this process has
	// not been launched through it, then we probably want to exit here and let
	// systemd launch appimaged. We need to set up systemd to be able to do that
	// in case it is not already set up this way.

	if !CheckIfRunningSystemd() {
		log.Println("This system is not running systemd")
		return
	}
	log.Println("Manually launched, not by systemd. Check if enabled in systemd...")

	if _, err := os.Stat("/etc/systemd/user/appimaged.service"); os.IsNotExist(err) {
		log.Println("/etc/systemd/user/appimaged.service does not exist")
		installServiceFileInHome()
	}
	prc := exec.Command("systemctl", "--user", "status", "appimaged")
	out, err := prc.CombinedOutput()
	if err != nil {
		log.Println(prc.String())
		log.Println(err)
		// Note that if the service is stopped, we get an error exit code
		// with "exit status 3", hence this must not be fatal here
	}
	output := strings.TrimSpace(string(out))
	if strings.Contains(output, " enabled; ") {
		log.Println("Restarting via systemd...")
		prc := exec.Command("systemctl", "--user", "restart", "appimaged")
		_, err := prc.CombinedOutput()
		if err != nil {
			log.Println(prc.String())
			log.Println(err)
		}
	} else {
		log.Println("Enabling systemd service...")
		prc := exec.Command("systemctl", "--user", "enable", "appimaged")
		_, err := prc.CombinedOutput()
		if err != nil {
			log.Println(prc.String())
			log.Println(err)
		}
		log.Println("Starting systemd service...")
		prc = exec.Command("systemctl", "--user", "restart", "appimaged")
		_, err = prc.CombinedOutput()
		if err != nil {
			log.Println(prc.String())
			log.Println(err)
		} else {
			log.Println("appimaged should now be running via systemd. To check this, run")
			log.Println("/usr/bin/systemctl -l --no-pager --user status appimaged")
		}
	}

}

// CheckIfRunningSystemd returns true if pid 1 is (a symlink to) systemd,
// otherwise false
func CheckIfRunningSystemd() bool {
	prc := exec.Command("ps", "-p", "1", "-o", "comm=")
	out, err := prc.Output()
	if err != nil {
		log.Println(prc.String())
		log.Println(err)
		return (false)
	}
	if strings.TrimSpace(string(out)) == "systemd" {
		return true
	}
	return false
}

// installServiceFileInHome installs a service file for the currently running
// AppImage in $XDG_DATA_HOME/systemd/user or $HOME/.local/share/systemd/user
func installServiceFileInHome() {
	var err error
	home, _ := os.UserHomeDir()
	// Note that https://www.freedesktop.org/software/systemd/man/systemd.unit.html
	// says $XDG_CONFIG_HOME/systemd/user or $HOME/.config/systemd/user
	// Units of packages that have been installed in the home directory
	// ($XDG_CONFIG_HOME is used if set, ~/.config otherwise)
	var pathToServiceDir string
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		log.Println("Creating $XDG_CONFIG_HOME/systemd/user/appimaged.service")
		err = os.MkdirAll(xdg.ConfigHome+"/systemd/user/", os.ModePerm)
		if err == nil {
			pathToServiceDir = xdg.ConfigHome + "/systemd/user/"
		} else {
			helpers.LogError("Failed making directory for service files", err)
			return
		}
	} else {
		log.Println("Creating ~/.config/systemd/user/appimaged.service")
		err = os.MkdirAll(home+"/.config/systemd/user/", os.ModePerm)
		if err == nil {
			pathToServiceDir = home + "/.config/systemd/user/"
		} else {
			helpers.LogError("Failed making directory for service files", err)
			return
		}
	}
	exe := thisai.Path
	if verbose {
		exe += " -v"
	}
	if *quiet {
		exe += " -q"
	}
	if *overwrite {
		exe += " -o"
	}
	if *clean {
		exe += " -c"
	}
	if *noZeroconf {
		exe += " -nz"
	}
	if *updateService {
		exe += " -u"
	}
	d1 := []byte(`[Unit]
Description=AppImage system integration daemon
After=syslog.target network.target

[Service]
Type=simple
ExecStart=` + exe + ` service

LimitNOFILE=65536

RestartSec=3
Restart=always

[Install]
WantedBy=default.target`)
	err = syncWriteFile(pathToServiceDir+"appimaged.service", d1, 0644)
	helpers.LogError("Error writing service file", err)

	prc := exec.Command("systemctl", "--user", "daemon-reload")
	_, err = prc.CombinedOutput()
	if err != nil {
		log.Println(prc.String())
		log.Println(err)
	}

}

// syncWritFile is a reproduction of os.WriteFile,
// with an additional call to f.Sync() before f.Close()
func syncWriteFile(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err = f.Write(data); err != nil {
		return err
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return f.Close()
}
