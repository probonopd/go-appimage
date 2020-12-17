package goappimage

import (
	"os/exec"
	"path"
	"strings"
)

//Tries to get the location the symlink at filepath is pointing to. If file is not a symlink, returns filepath. Only for type 1
func (ai AppImage) getSymlinkLocation(filepath string) (string, error) {
	cmd := exec.Command("bsdtar", "-f", ai.path, "-tv", filepath)
	wrt, err := runCommand(cmd)
	if err != nil {
		return filepath, err
	}
	output := strings.TrimSuffix(string(wrt.Bytes()), "\n")
	if index := strings.Index(output, "->"); index != -1 { //signifies symlink
		symlinkedFile := output[index+3:]
		if strings.HasPrefix(symlinkedFile, "/") {
			return filepath, nil //we can't help with absolute symlinks...
		}
		return ai.getSymlinkLocation(path.Dir(filepath) + "/" + symlinkedFile)
	}
	return filepath, nil
}
