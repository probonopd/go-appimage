package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
)
import "debug/elf"
import "github.com/probonopd/go-appimage/internal/helpers"

var allLibs []string

var AppRunData = `#!/bin/sh

HERE="$(dirname "$(readlink -f "${0}")")"

MAIN=$(grep -r "^Exec=.*" "$HERE"/*.desktop | head -n 1 | cut -d "=" -f 2 | cut -d " " -f 1)

############################################################################################
# Use bundled paths
############################################################################################

export PATH="${HERE}"/usr/bin/:"${HERE}"/usr/sbin/:"${HERE}"/usr/games/:"${HERE}"/bin/:"${HERE}"/sbin/:"${PATH}"
export XDG_DATA_DIRS="${HERE}"/usr/share/:"${XDG_DATA_DIRS}"

############################################################################################
# Use bundled Python
############################################################################################

if [ -e "${HERE}"/usr/share/pyshared/ ] ; then
  export PYTHONPATH="${HERE}"/usr/share/pyshared/:"${PYTHONPATH}"
  export PYTHONHOME="${HERE}"/usr/
fi

############################################################################################
# Use bundled Tcl/Tk
############################################################################################

if [ -e "${HERE}"/usr/share/tcltk/tcl8.6 ] ; then
  export TCL_LIBRARY="${HERE}"/usr/share/tcltk/tcl8.6:$TCL_LIBRARY:$TK_LIBRARY
  export TK_LIBRARY="${HERE}"/usr/share/tcltk/tk8.6:$TK_LIBRARY:$TCL_LIBRARY
fi

############################################################################################
# Make it look more native on Gtk+ based systems
############################################################################################

case "${XDG_CURRENT_DESKTOP}" in
    *GNOME*|*gnome*)
        export QT_QPA_PLATFORMTHEME=gtk2
esac

############################################################################################
# Run experimental bundle that bundles everything if a private ld-linux-x86-64.so.2 is there
# This allows the bundle to run even on older systems than the one it was built on
############################################################################################

if [ -e "$HERE/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" ] ; then
  echo "Run experimental bundle that bundles everything"
  export GCONV_PATH="$HERE/usr/lib/x86_64-linux-gnu/gconv"
  export FONTCONFIG_FILE="$HERE/etc/fonts/fonts.conf"
  export LIBRARY_PATH="$HERE/usr/lib":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/lib":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/i386-linux-gnu":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/lib/i386-linux-gnu":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/i386-linux-gnu/pulseaudio":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/i386-linux-gnu/alsa-lib":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/x86_64-linux-gnu":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/lib/x86_64-linux-gnu":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/x86_64-linux-gnu/pulseaudio":$LIBRARY_PATH
  export LIBRARY_PATH="$HERE/usr/lib/x86_64-linux-gnu/alsa-lib":$LIBRARY_PATH
  export LIBRARY_PATH=$GDK_PIXBUF_MODULEDIR:$LIBRARY_PATH # Otherwise getting "Unable to load image-loading module"
  exec "${HERE}/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" --inhibit-cache --library-path "${LIBRARY_PATH}" "${MAIN}" "$@"
else
  exec $(which "${MAIN}") "$@"
fi
`

type ELF struct {
	path     string
	relpaths []string
	rpath    string
}

/*
   man ld.so says:

   If a shared object dependency does not contain a slash, then it is searched for in the following order:

   o  Using the directories specified in the DT_RPATH dynamic section attribute of the binary if present and DT_RUNPATH attribute does not  exist.   Use
      of DT_RPATH is deprecated.

   o  Using  the environment variable LD_LIBRARY_PATH, unless the executable is being run in secure-execution mode (see below), in which case this vari‐
      able is ignored.

   o  Using the directories specified in the DT_RUNPATH dynamic section attribute of the binary if present.  Such directories are searched only to  find
      those  objects  required  by DT_NEEDED (direct dependencies) entries and do not apply to those objects' children, which must themselves have their
      own DT_RUNPATH entries.  This is unlike DT_RPATH, which is applied to searches for all children in the dependency tree.

   o  From the cache file /etc/ld.so.cache, which contains a compiled list of candidate shared objects previously found in the augmented  library  path.
      If,  however, the binary was linked with the -z nodeflib linker option, shared objects in the default paths are skipped.  Shared objects installed
      in hardware capability directories (see below) are preferred to other shared objects.

      NOTE: Clear Linux* OS has this in /var/cache/ldconfig/ld.so.cache

   o  In the default path /lib, and then /usr/lib.  (On some 64-bit architectures, the default paths for 64-bit shared  objects  are  /lib64,  and  then
      /usr/lib64.)  If the binary was linked with the -z nodeflib linker option, this step is skipped.
*/

func main() {

	// Check for needed files on $PATH
	helpers.AddDirsToPath([]string{helpers.Here()})
	tools := []string{"patchelf", "desktop-file-validate"}
	for _, t := range tools {
		_, err := exec.LookPath(t)
		if err != nil {
			fmt.Println("Required helper tool", t, "missing")
			os.Exit(1)
		}
	}

	if len(os.Args) < 2 {
		fmt.Println("Please supply the path to a desktop file in an FHS-like AppDir")
		fmt.Println("a FHS-like structure, e.g.:")
		fmt.Println(os.Args[0], "appdir/usr/share/applications/myapp.desktop")
		os.Exit(1)
	}

	appdir, err := helpers.NewAppDir(os.Args[1])
	if err != nil {
		helpers.PrintError("AppDir", err)
		os.Exit(1)
	}

	fmt.Println("Finding all dependency libraries...")

	err = getDeps(appdir.MainExecutable)

	helpers.PrintError("getDeps", err)

	fmt.Println("Number of libraries:", len(allLibs))

	fmt.Println("allLibs", allLibs)

	fmt.Println("Copying all required libraries into the AppDir...")

	// Copy all libraries into the AppDir, at the location in which they originally were in the host system
	// TODO: Consider copying to one predetermined location inside the AppDir, e.g., usr/lib
	// to simplify things. Since it might break on things like dlopen(), we are not doing it so far

	// Add all executables and pre-existing ELFs in the AppDir to allELFsInAppDir
	// independent of whether they have execute permissions or not
	allelfs, err := findAllExecutablesAndLibraries(appdir.Path)
	if err != nil {
		helpers.PrintError("findAllExecutablesAndLibraries:", err)
	}
	var allELFsInAppDir []ELF
	for _, elfpath := range allelfs {
		elfobj := ELF{}
		elfobj.path = elfpath
		allELFsInAppDir = append(allELFsInAppDir, elfobj)
	}
	fmt.Println("len(allELFsInAppDir), FIXME: why is this 0:", len(allELFsInAppDir))

	// Copy the libraries determined by our ldd replacement into the AppDir, and add them to
	// allELFsInAppDir if they are not there yet
	for _, lib := range allLibs {
		os.MkdirAll(filepath.Dir(appdir.Path+"/"+lib), 0755)
		if helpers.Exists(appdir.Path+"/"+lib) == false {
			helpers.CopyFile(lib, appdir.Path+"/"+lib)
		}
		allELFsInAppDir = append(allELFsInAppDir, ELF{path: appdir.Path + "/" + lib})
	}

	// Find out in which directories we now actually have libraries
	allDirectoriesWithLibraries, err := getAllDirectoriesWithLibraries(appdir.Path)
	helpers.PrintError("allDirectoriesWithLibraries", err)
	fmt.Println("allDirectoriesWithLibraries:", allDirectoriesWithLibraries)

	fmt.Println("Computing relative locations to directories containing libraries...")

	fmt.Println("len(allELFsInAppDir):", len(allELFsInAppDir))

	fmt.Println("Patching ELF files to find libraries in relative locations...")

	// For each ELF in the AppDir, compute the relative locations to directories containing libraries for the rpath
	for i, l := range allELFsInAppDir {
		// fmt.Println(l.path)
		os.Chmod(l.path, 0755)
		for _, libdir := range allDirectoriesWithLibraries {
			rel, err := filepath.Rel(filepath.Dir(l.path), libdir)
			if err == nil && rel != "." {
				allELFsInAppDir[i].relpaths = append(allELFsInAppDir[i].relpaths, rel)
			}
		}
		allELFsInAppDir[i].rpath = "$ORIGIN:$ORIGIN/" + strings.Join(allELFsInAppDir[i].relpaths, ":$ORIGIN/")
		// fmt.Println("rpath for", allELFsInAppDir[i].path, allELFsInAppDir[i].rpath)

		// Call patchelf to find out whether the ELF already has an rpath set
		cmd := exec.Command("patchelf", "--print-rpath", allELFsInAppDir[i].path)
		// fmt.Println(cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println(allELFsInAppDir[i].path, strings.TrimSpace(string(out)))
			helpers.PrintError("patchelf --print-rpath "+allELFsInAppDir[i].path, err)
			os.Exit(1)
		}
		// fmt.Println(string(out))
		if strings.Contains(allELFsInAppDir[i].path, "ld-linux") {
			fmt.Println("Not writing rpath to", allELFsInAppDir[i].path, "because ld-linux apparently does not like this")
		} else if strings.HasPrefix(string(out), "$") == false && strings.Contains(allELFsInAppDir[i].path, "ld-linux") {
			// Call patchelf to set the rpath
			cmd = exec.Command("patchelf", "--set-rpath", allELFsInAppDir[i].rpath, allELFsInAppDir[i].path)
			fmt.Println(cmd.Args)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(allELFsInAppDir[i].path, strings.TrimSpace(string(out)))
				helpers.PrintError("patchelf --set-rpath "+allELFsInAppDir[i].path, err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Not writing rpath to", allELFsInAppDir[i].path, "because it already starts with $")
		}

	}

	fmt.Println("TODO: Patching ld-linux... xxxxxxxxxxxxxxxxxxxx")

	fmt.Println("TODO: Copying in glibc parts... xxxxxxxxxxxxxxxxxxxx")

	fmt.Println("Adding AppRun...")

	err = ioutil.WriteFile(appdir.Path+"/AppRun", []byte(AppRunData), 0755)
	if err != nil {
		helpers.PrintError("write AppRun", err)
		os.Exit(1)
	}

}

// findAllExecutablesAndLibraries returns all ELF libraries and executables
// found in directory, and error
func findAllExecutablesAndLibraries(directory string) ([]string, error) {
	var allExecutablesAndLibraries []string
	filepath.Walk(directory, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check if it is a regular file (not dir) and, according to its name, is a shared library
		if info.Mode().IsRegular() && strings.Contains(path, ".so") {
			allExecutablesAndLibraries = helpers.AppendIfMissing(allExecutablesAndLibraries, path)
		}

		// Add all executable/ELF files
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			defer f.Close()
			if err == nil {
				if helpers.CheckMagicAtOffset(f, "454c46", 1) == true {
					allExecutablesAndLibraries = helpers.AppendIfMissing(allExecutablesAndLibraries, path)
				}
			}
		}

		return nil
	})
	return allExecutablesAndLibraries, nil
}

// getAllDirectoriesWithLibraries returns all directories under the supplied
// diectory in which .so files reside, and error. This is useful to compute the rpaths needed
func getAllDirectoriesWithLibraries(directory string) ([]string, error) {
	var allDirectoriesWithLibraries []string
	filepath.Walk(directory, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// check if it is a regular file (not dir)
		if info.Mode().IsRegular() && strings.Contains(path, ".so") {

			allDirectoriesWithLibraries = helpers.AppendIfMissing(allDirectoriesWithLibraries, filepath.Dir(path))
		}
		return nil
	})
	return allDirectoriesWithLibraries, nil
}

func getDeps(binaryOrLib string) error {
	var libs []string

	if helpers.Exists(binaryOrLib) == false {
		return errors.New("binary does not exist: " + binaryOrLib)
	}

	e, err := elf.Open(binaryOrLib)
	// fmt.Println("getDeps", binaryOrLib)
	helpers.PrintError("elf.Open", err)

	// ImportedLibraries returns the names of all libraries
	// referred to by the binary f that are expected to be
	// linked with the binary at dynamic link time.
	libs, err = e.ImportedLibraries()
	helpers.PrintError("e.ImportedLibraries", err)

	for _, lib := range libs {
		s, err := findLibrary(lib)
		if err != nil {
			return err
		}
		if helpers.SliceContains(allLibs, s) == true {
			continue
		} else {
			libPath, err := findLibrary(lib)
			helpers.PrintError("findLibrary", err)
			allLibs = append(allLibs, libPath)
			err = getDeps(libPath)
			helpers.PrintError("findLibrary", err)
		}
	}
	return nil
}

func findLibrary(filename string) (string, error) {

	// Look for libraries in the same locations in which the system looks for libraries
	// TODO: Instead of hardcoding libraryLocations, get them from the system - see the comment at the top xxxxxxxxx
	libraryLocations := []string{"/usr/lib64", "/lib64", "/usr/lib", "/lib"}

	// Also look for libraries in in LD_LIBRARY_PATH
	ldpstr := os.Getenv("LD_LIBRARY_PATH")
	ldps := strings.Split(ldpstr, ":")
	for _, ldp := range ldps {
		libraryLocations = append(libraryLocations, ldp)
	}

	// TODO: Parse elf for pre-existing rpath/runpath and consider those locations as well

	// Try to find the library in one of those locations
	for _, libraryLocation := range libraryLocations {
		if helpers.Exists(libraryLocation + "/" + filename) {
			return libraryLocation + "/" + filename, nil
		}
	}
	return "", errors.New("did not find library " + filename)
}

func NewLibrary(path string) ELF {
	lib := ELF{}
	lib.path = path
	return lib
}
