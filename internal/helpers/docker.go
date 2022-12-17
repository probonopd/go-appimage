package helpers

import (
	"log"
	"os"
	"strings"
)

// CheckRunningWithinDocker  checks if the tool is running within a Docker container
// and warn the user of passing Environment variables to the container
func CheckRunningWithinDocker() bool {
	// Detect if we are running inside Docker; https://github.com/AppImage/AppImageKit/issues/912
	// If the file /.dockerenv exists, and/or if /proc/1/cgroup begins with /lxc/ or /docker/
	res, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		// Do not exit if os.ReadFile("/proc/1/cgroup") fails. This happens, e.g., on FreeBSD
		if strings.HasPrefix(string(res), "/lxc") || strings.HasPrefix(string(res), "/docker") || Exists("/.dockerenv") == true {
			log.Println("Running inside Docker. Please make sure that the environment variables from Travis CI")
			log.Println("available inside Docker if you are running on Travis CI.")
			log.Println("This can be achieved by using something along the lines of 'docker run --env-file <(env)'.")
			log.Println("Please see https://github.com/docker/cli/issues/2210.")
			return true
		}
	}
	return false
}
