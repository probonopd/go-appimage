package helpers

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PrintError prints error, prefixed by a string that explains the context
func PrintError(context string, e error) {
	if e != nil {
		os.Stderr.WriteString("ERROR " + context + ": " + e.Error() + "\n")
	}
}

// LogError logs error, prefixed by a string that explains the context
func LogError(context string, e error) {
	if e != nil {
		l := log.New(os.Stderr, "", 1)
		l.Println("ERROR " + context + ": " + e.Error())
	}
}

// Here returns the location of the executable based on os.Args[0]
func Here() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
		return ""
	}
	return (dir)
}

// AddHereToPath adds the location of the executable to the $PATH
func AddHereToPath() {
	// The directory we run from is added to the $PATH so that we find helper
	// binaries there, too
	os.Setenv("PATH", Here()+":"+os.Getenv("PATH"))
	log.Println("main: PATH:", os.Getenv("PATH"))
}

// IsCommandAvailable returns true if a file is on the $PATH
func IsCommandAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// FilesWithSuffixInDirectory returns the files in a given directory with the given filename extension, and err
func FilesWithSuffixInDirectory(directory string, extension string) []string {
	var foundfiles []string
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return foundfiles
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), extension) {
			foundfiles = append(foundfiles, directory+"/"+file.Name())
		}
	}
	return foundfiles
}
