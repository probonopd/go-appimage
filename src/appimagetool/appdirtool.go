package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
)
import "debug/elf"
import "github.com/probonopd/go-appimage/internal/helpers"
import "github.com/otiai10/copy"

var allELFs []string
var libraryLocations []string // All directories in the host system that may contain libraries

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
# If .ui files are in the AppDir, then chances are that we need to cd into usr/
# because we may have had to patch the absolute paths away in the binary
############################################################################################

UIFILES=$(find "$HERE" -name "*.ui")
if [ ! -z "$UIFILES" ] ; then
  cd "$HERE/usr"
fi

############################################################################################
# Run experimental bundle that bundles everything if a private ld-linux-x86-64.so.2 is there
# This allows the bundle to run even on older systems than the one it was built on
############################################################################################

MAIN_BIN=$(find "$HERE/usr/bin" -name "$MAIN" | head -n 1)
LD_LINUX=$(find "$HERE" -name 'ld-linux-*.so.*' | head -n 1)
if [ -e "$LD_LINUX" ] ; then
  echo "Run experimental bundle that bundles everything"
  export GCONV_PATH="$HERE/usr/lib/gconv"
  export FONTCONFIG_FILE="$HERE/etc/fonts/fonts.conf"
  export GTK_EXE_PREFIX="$HERE/usr"
  export GTK_THEME=Default # This one should be bundled so that it can work on systems without Gtk
  export GDK_PIXBUF_MODULEDIR=$(find "$HERE" -name loaders -type d -path '*gdk-pixbuf*')
  export GDK_PIXBUF_MODULE_FILE=$(find "$HERE" -name loaders.cache -type f -path '*gdk-pixbuf*') # Patched to contain no paths
  # export LIBRARY_PATH=$GDK_PIXBUF_MODULEDIR # Otherwise getting "Unable to load image-loading module"
  export XDG_DATA_DIRS="${HERE}"/usr/share/:"${XDG_DATA_DIRS}"
  export PERLLIB="${HERE}"/usr/share/perl5/:"${HERE}"/usr/lib/perl5/:"${PERLLIB}"
  export GSETTINGS_SCHEMA_DIR="${HERE}"/usr/share/glib-2.0/schemas/:"${GSETTINGS_SCHEMA_DIR}"
  under_GST_PLUGIN_SYSTEM_PATH=$(find "${HERE}" -name "libgstpng.so" -type f | head -n 1)
  if [ ! -z "$under_GST_PLUGIN_SYSTEM_PATH"" ] ; then
    export GST_PLUGIN_SYSTEM_PATH=$(dirname under_GST_PLUGIN_SYSTEM_PATH)
  fi
  export QT_PLUGIN_PATH="${HERE}"/usr/lib/qt4/plugins/:"${HERE}"/usr/lib/i386-linux-gnu/qt4/plugins/:"${HERE}"/usr/lib/x86_64-linux-gnu/qt4/plugins/:"${HERE}"/usr/lib32/qt4/plugins/:"${HERE}"/usr/lib64/qt4/plugins/:"${HERE}"/usr/lib/qt5/plugins/:"${HERE}"/usr/lib/i386-linux-gnu/qt5/plugins/:"${HERE}"/usr/lib/x86_64-linux-gnu/qt5/plugins/:"${HERE}"/usr/lib32/qt5/plugins/:"${HERE}"/usr/lib64/qt5/plugins/:"${QT_PLUGIN_PATH}"
  # exec "${LD_LINUX}" --inhibit-cache --library-path "${LIBRARY_PATH}" "${MAIN_BIN}" "$@"
  exec "${LD_LINUX}" --inhibit-cache "${MAIN_BIN}" "$@"
else
  echo "Bundle has issues, cannot launch"
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

func AppDirDeploy(path string) {

	appdir, err := helpers.NewAppDir(path)
	if err != nil {
		helpers.PrintError("AppDir", err)
		os.Exit(1)
	}

	log.Println("Gathering all required libraries for the AppDir...")
	determineELFsInDirTree(appdir, appdir.Path)

	// If there is a .so with the name libgdk_pixbuf inside the AppDir, then we need to
	// bundle Gdk pixbuf loaders without which the bundled Gtk does not work
	// cp /usr/lib/x86_64-linux-gnu/gdk-pixbuf-*/*/loaders/* usr/lib/x86_64-linux-gnu/gdk-pixbuf-*/*/loaders/
	// cp /usr/lib/x86_64-linux-gnu/gdk-pixbuf-*/*/loaders.cache usr/lib/x86_64-linux-gnu/gdk-pixbuf-*/*/ -
	// this file must also be patched not to contain paths to the libraries

	for _, lib := range allELFs {
		if strings.HasPrefix(filepath.Base(lib), "libgdk_pixbuf") {
			log.Println("Determining Gdk pixbuf loaders (for GDK_PIXBUF_MODULEDIR and GDK_PIXBUF_MODULE_FILE)...")
			locs, err := findWithPrefixInLibraryLocations("gdk-pixbuf")
			if err != nil {
				log.Println("Could not find Gdk pixbuf loaders")
				os.Exit(1)
			} else {
				for _, loc := range locs {
					determineELFsInDirTree(appdir, loc)

					// We need to patch away the path to libpixbufloader-png.so from the file loaders.cache, similar to:
					// sed -i -e 's|/usr/lib/x86_64-linux-gnu/gdk-pixbuf-2.0/2.10.0/loaders/||g' usr/lib/x86_64-linux-gnu/gdk-pixbuf-*/*/loaders.cache
					// the patched file must not contain paths to the libraries
					loadersCaches := helpers.FilesWithSuffixInDirectoryRecursive(loc, "loaders.cache")
					if len(loadersCaches) < 1 {
						helpers.PrintError("loadersCaches", errors.New("could not find loaders.cache"))
						os.Exit(1)
					}

					err = copy.Copy(loadersCaches[0], appdir.Path+loadersCaches[0])
					if err != nil {
						helpers.PrintError("Could not copy loaders.cache", err)
						os.Exit(1)
					}

					whatToPatchAway := helpers.FilesWithSuffixInDirectoryRecursive(loc, "libpixbufloader-png.so")
					if len(whatToPatchAway) < 1 {
						helpers.PrintError("whatToPatchAway", errors.New("could not find directory that contains libpixbufloader-png.so"))
						os.Exit(1)
					}

					log.Println("Patching", appdir.Path+loadersCaches[0], "removing", filepath.Dir(whatToPatchAway[0])+"/")
					err = PatchFile(appdir.Path+loadersCaches[0], filepath.Dir(whatToPatchAway[0])+"/", "")
					if err != nil {
						helpers.PrintError("PatchFile loaders.cache", err)
						os.Exit(1)
					}

				}
			}
			break
		}
	}

	// GStreamer
	for _, lib := range allELFs {
		if strings.HasPrefix(filepath.Base(lib), "libgstreamer-1.0") {
			log.Println("Bundling GStreamer 1.0 directory (for GST_PLUGIN_PATH)...")
			locs, err := findWithPrefixInLibraryLocations("gstreamer-1.0")
			if err != nil {
				log.Println("Could not find GStreamer 1.0 directory")
				os.Exit(1)
			} else {
				log.Println("Bundling dependencies of GStreamer 1.0 directory...")
				determineELFsInDirTree(appdir, locs[0])
			}
			break
		}
	}

	// FIXME: This is not going to scale, every distribution is cooking their own soup,
	// we need to determine the location of gst-plugin-scanner dynamically by parsing it out of libgstreamer-1.0
	gstPluginScannerCandidates := []string{"/usr/libexec/gstreamer-1.0/gst-plugin-scanner", // Clear Linux* OS
		"/usr/lib/x86_64-linux-gnu/gstreamer1.0/gstreamer-1.0/gst-plugin-scanner"} // sic! Ubuntu 18.04
	for _, cand := range gstPluginScannerCandidates {
		if helpers.Exists(cand) {
			log.Println("Determining gst-plugin-scanner...")
			determineELFsInDirTree(appdir, cand)
			break
		}
	}

	// Gtk 3 modules/plugins
	// If there is a .so with the name libgtk-3 inside the AppDir, then we need to
	// bundle Gdk modules/plugins
	deployGtkDirectory(appdir, 3)

	// Gtk 2 modules/plugins
	// Same as above, but for Gtk 2
	deployGtkDirectory(appdir, 2)

	log.Println("Patching ld-linux...")

	cmd := exec.Command("patchelf", "--print-interpreter", appdir.MainExecutable)
	out, err := cmd.CombinedOutput()
	if err != nil {
		helpers.PrintError("patchelf --print-interpreter", err)
		os.Exit(1)
	}
	ldLinux := strings.TrimSpace(string(out))

	if helpers.Exists(appdir.Path+ldLinux) == false {

		// ld-linux might be a symlink; hence we first need to resolve it
		src, err := os.Readlink(ldLinux)
		if err != nil {
			helpers.PrintError("Could not get the location of ld-linux", err)
			os.Exit(1)
		}

		err = copy.Copy(src, appdir.Path+ldLinux)
		if err != nil {
			helpers.PrintError("Could not copy ld-linux", err)
			os.Exit(1)
		}
	}

	// Do what we do in the Scribus AppImage script, namely
	// sed -i -e 's|/usr|/xxx|g' lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
	err = PatchFile(appdir.Path+ldLinux, "/usr", "/xxx")
	if err != nil {
		helpers.PrintError("PatchFile", err)
		os.Exit(1)
	}

	log.Println("Determining gconv (for GCONV_PATH)...")
	// Search in all of the system's library directories for a directory called gconv
	// and put it into the a location which matches the GCONV_PATH we export in AppRun
	gconvs, err := findWithPrefixInLibraryLocations("gconv")
	log.Println("TODO: Handle", gconvs[0])
	if err == nil {
		// Target location must match GCONV_PATH exported in AppRun
		determineELFsInDirTree(appdir, gconvs[0])
	}

	if helpers.Exists(appdir.Path + "/usr/share/glib-2.0/schemas") {
		log.Println("Compiling glib-2.0 schemas...")
		// Do what we do in pkg2appimage
		// Compile GLib schemas if the subdirectory is present in the AppImage
		// AppRun has to export GSETTINGS_SCHEMA_DIR for this to work
		if helpers.Exists(appdir.Path + "/usr/share/glib-2.0/schemas") {
			cmd := exec.Command("glib-compile-schemas", ".")
			cmd.Dir = appdir.Path + "/usr/share/glib-2.0/schemas"
			err = cmd.Run()
			if err != nil {
				helpers.PrintError("Run glib-compile-schemas", err)
				os.Exit(1)
			}
		}
	}

	if helpers.Exists(appdir.Path+"/etc/fonts") == false {
		log.Println("Adding fontconfig symlink... (is this really the right thing to do?)")
		err = os.MkdirAll(appdir.Path+"/etc/fonts", 0755)
		if err != nil {
			helpers.PrintError("MkdirAll", err)
			os.Exit(1)
		}
		err = os.Symlink("/etc/fonts/fonts.conf", appdir.Path+"/etc/fonts/fonts.conf")
		if err != nil {
			helpers.PrintError("MkdirAll", err)
			os.Exit(1)
		}
	}

	// Check for the presence of Gtk .ui files
	uifiles := helpers.FilesWithSuffixInDirectoryRecursive(appdir.Path, ".ui")
	if len(uifiles) > 0 {
		log.Println("Gtk .ui files found. Need to take care to have them loaded from a relative rather than absolute path")
		log.Println("TODO: Check if they are at hardcoded absolute paths in the application and if yes, patch")
		var dirswithUiFiles []string
		for _, uifile := range uifiles {
			dirswithUiFiles = helpers.AppendIfMissing(dirswithUiFiles, filepath.Dir(uifile))
			err = PatchFile(appdir.MainExecutable, "/usr", "././")
			if err != nil {
				helpers.PrintError("PatchFile", err)
				os.Exit(1)
			}
		}
		log.Println("Directories with .ui files:", dirswithUiFiles)
	}

	log.Println("Adding AppRun...")

	err = ioutil.WriteFile(appdir.Path+"/AppRun", []byte(AppRunData), 0755)
	if err != nil {
		helpers.PrintError("write AppRun", err)
		os.Exit(1)
	}

	fmt.Println("")
	log.Println("libraryLocations:")
	for _, lib := range libraryLocations {
		fmt.Println(lib)
	}

	fmt.Println("")
	// This is used when calculating the rpath that gets written into the ELFs as they are copied into the AppDir
	// and when modifying the ELFs that were pre-existing in the AppDir so that they become aware of the other locations
	var libraryLocationsInAppDir []string
	for _, lib := range libraryLocations {
		if strings.HasPrefix(lib, appdir.Path) == false {
			lib = appdir.Path + lib
		}
		libraryLocationsInAppDir = helpers.AppendIfMissing(libraryLocationsInAppDir, lib)
	}

	fmt.Println("")
	log.Println("libraryLocationsInAppDir:")
	for _, lib := range libraryLocationsInAppDir {
		fmt.Println(lib)
	}

	fmt.Println("")
	log.Println("allELFs:")
	for _, lib := range allELFs {
		fmt.Println(lib)
	}

	log.Println("Only after this point should we start copying around any ELFs")

	log.Println("Copying in and patching ELFs which are not already in the AppDir...")

	for _, lib := range allELFs {

		if strings.HasPrefix(filepath.Base(lib), "libnvidia") == true {
			log.Println("Skipping", lib, "because it crashes when running on older systems")
			continue
		}

		if strings.HasPrefix(lib, appdir.Path) == false && helpers.Exists(appdir.Path+"/"+lib) == false {

			err = helpers.CopyFile(lib, appdir.Path+"/"+lib)
			if err != nil {
				log.Println(appdir.Path+"/"+lib, "could not be copied:", err)
				os.Exit(1)
			}

			// TODO: Copy license file for lib

		}

		patchRpathsInElf(appdir, libraryLocationsInAppDir, lib)

	}
}

func patchRpathsInElf(appdir helpers.AppDir, libraryLocationsInAppDir []string, path string) {

	if strings.HasPrefix(path, appdir.Path) == false {
		path = filepath.Clean(appdir.Path + "/" + path)
	}
	var newRpathStringForElf string
	var newRpathStrings []string
	for _, libloc := range libraryLocationsInAppDir {

		relpath, err := filepath.Rel(filepath.Dir(path), libloc)
		if err != nil {
			helpers.PrintError("Could not compute relative path", err)
		}
		newRpathStrings = append(newRpathStrings, "$ORIGIN/"+filepath.Clean(relpath))
	}
	newRpathStringForElf = strings.Join(newRpathStrings, ":")
	// fmt.Println("Computed newRpathStringForElf:", appdir.Path+"/"+lib, newRpathStringForElf)

	if strings.HasPrefix(filepath.Base(path), "ld-linux") == true {
		log.Println("Not writing rpath in", path, "because its name starts with ld-linux")
		return
	}

	// Call patchelf to set the rpath
	log.Println("Rewriting rpath of", path)
	cmd := exec.Command("patchelf", "--set-rpath", newRpathStringForElf, path)
	// log.Println(cmd.Args)
	_, err := cmd.CombinedOutput()
	if err != nil {
		helpers.PrintError("patchelf --set-rpath "+path, err)
		os.Exit(1)
	}
}

func deployGtkDirectory(appdir helpers.AppDir, gtkVersion int) {
	for _, lib := range allELFs {
		if strings.HasPrefix(filepath.Base(lib), "libgtk-"+strconv.Itoa(gtkVersion)) {
			log.Println("Bundling Gtk", strconv.Itoa(gtkVersion), "directory (for GTK_EXE_PREFIX)...")
			locs, err := findWithPrefixInLibraryLocations("gtk-" + strconv.Itoa(gtkVersion))
			if err != nil {
				log.Println("Could not find Gtk", strconv.Itoa(gtkVersion), "directory")
				os.Exit(1)
			} else {
				for _, loc := range locs {
					log.Println("Bundling dependencies of Gtk", strconv.Itoa(gtkVersion), "directory...")
					determineELFsInDirTree(appdir, loc)
					log.Println("Bundling Default theme for Gtk", strconv.Itoa(gtkVersion), "(for GTK_THEME=Default)...")
					err = copy.Copy("/usr/share/themes/Default/gtk-"+strconv.Itoa(gtkVersion)+".0", appdir.Path+"/usr/share/themes/Default/gtk-"+strconv.Itoa(gtkVersion)+".0")
					if err != nil {
						helpers.PrintError("Copy", err)
						os.Exit(1)
					}

					/*
						log.Println("Bundling icons for Default theme...")
						err = copy.Copy("/usr/share/icons/Adwaita", appdir.Path+"/usr/share/icons/Adwaita")
						if err != nil {
							helpers.PrintError("Copy", err)
							os.Exit(1)
						}
					*/
				}
			}
			break
		}
	}
}

// appendLib appends library in path to allELFs and adds its location as well as any pre-existing rpaths to libraryLocations
func appendLib(path string) {

	// Find out whether there are pre-existing rpaths and if so, add them to libraryLocations
	// so that we can find libraries there, too
	// See if the library had a pre-existing rpath that did not start with $. If so, replace it by one that
	// points to the equal location as the original but inside the AppDir
	rpaths, err := readRpaths(path)
	if err != nil {
		helpers.PrintError("Could not determine rpath", err)
		os.Exit(1)
	}

	for _, rpath := range rpaths {
		rpath = filepath.Clean(strings.Replace(rpath, "$ORIGIN", filepath.Dir(path), -1))
		if helpers.SliceContains(libraryLocations, rpath) == false && rpath != "" {
			log.Println("Add", rpath, "to the libraryLocations directories we search for libraries")
			libraryLocations = helpers.AppendIfMissing(libraryLocations, filepath.Clean(rpath))
		}
	}

	libraryLocations = helpers.AppendIfMissing(libraryLocations, filepath.Clean(filepath.Dir(path)))

	allELFs = helpers.AppendIfMissing(allELFs, path)
}

func determineELFsInDirTree(appdir helpers.AppDir, pathToDirTreeToBeDeployed string) {
	allelfs, err := findAllExecutablesAndLibraries(pathToDirTreeToBeDeployed)
	if err != nil {
		helpers.PrintError("findAllExecutablesAndLibraries", err)
	}

	// Find the libraries determined by our ldd replacement and add them to
	// allELFsUnderPath if they are not there yet
	for _, lib := range allelfs {
		appendLib(lib)
	}

	var allELFsUnderPath []ELF
	for _, elfpath := range allelfs {
		elfobj := ELF{}
		elfobj.path = elfpath
		allELFsUnderPath = append(allELFsUnderPath, elfobj)
		err = getDeps(elfpath)
		helpers.PrintError("getDeps", err)
	}
	log.Println("len(allELFsUnderPath):", len(allELFsUnderPath))

	// Find out in which directories we now actually have libraries
	log.Println("libraryLocations:", libraryLocations)
	log.Println("len(allELFs):", len(allELFs))
}

func readRpaths(path string) ([]string, error) {
	// Call patchelf to find out whether the ELF already has an rpath set
	cmd := exec.Command("patchelf", "--print-rpath", path)
	// log.Println(cmd.Args)
	out, err := cmd.CombinedOutput()
	rpathStringInELF := strings.TrimSpace(string(out))
	if rpathStringInELF == "" {
		return []string{}, err
	}
	rpaths := strings.Split(rpathStringInELF, ":")
	// log.Println("Determined", len(rpaths), "rpaths:", rpaths)
	return rpaths, err
}

// findAllExecutablesAndLibraries returns all ELF libraries and executables
// found in directory, and error
func findAllExecutablesAndLibraries(directory string) ([]string, error) {
	var allExecutablesAndLibraries []string
	filepath.Walk(directory, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		// Add ELF files
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

func getDeps(binaryOrLib string) error {
	var libs []string

	if helpers.Exists(binaryOrLib) == false {
		return errors.New("binary does not exist: " + binaryOrLib)
	}

	e, err := elf.Open(binaryOrLib)
	// log.Println("getDeps", binaryOrLib)
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
		if helpers.SliceContains(allELFs, s) == true {
			continue
		} else {
			libPath, err := findLibrary(lib)
			helpers.PrintError("findLibrary", err)
			appendLib(libPath)
			err = getDeps(libPath)
			helpers.PrintError("findLibrary", err)
		}
	}
	return nil
}

func findWithPrefixInLibraryLocations(prefix string) ([]string, error) {
	var found []string
	// Try to find the file or directory in one of those locations
	for _, libraryLocation := range libraryLocations {
		found = helpers.FilesWithPrefixInDirectory(libraryLocation, prefix)
		if len(found) > 0 {
			return found, nil
		}
	}
	return found, errors.New("did not find " + prefix)
}

func findLibrary(filename string) (string, error) {

	// Look for libraries in the same locations in which the system looks for libraries
	// TODO: Instead of hardcoding libraryLocations, get them from the system - see the comment at the top xxxxxxxxx
	locs := []string{"/usr/lib64", "/lib64", "/usr/lib", "/lib",
		// The following was determined on Ubuntu 18.04 using
		// $ find /etc/ld.so.conf.d/ -type f -exec cat {} \;
		"/usr/lib/x86_64-linux-gnu/libfakeroot",
		"/usr/local/lib",
		"/usr/local/lib/x86_64-linux-gnu",
		"/lib/x86_64-linux-gnu",
		"/usr/lib/x86_64-linux-gnu",
		"/lib32",
		"/usr/lib32"}

	for _, loc := range locs {
		libraryLocations = helpers.AppendIfMissing(libraryLocations, filepath.Clean(loc))
	}

	// Also look for libraries in in LD_LIBRARY_PATH
	ldpstr := os.Getenv("LD_LIBRARY_PATH")
	ldps := strings.Split(ldpstr, ":")
	for _, ldp := range ldps {
		if ldp != "" {
			libraryLocations = helpers.AppendIfMissing(libraryLocations, filepath.Clean(ldp))
		}
	}

	// TODO: find ld.so.cache on the system and use the locations contained therein, too

	// Somewhere else in this code we are parsing each elf for pre-existing rpath/runpath and consider those locations as well

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

// PatchFile patches file by replacing 'search' with 'replace', returns error.
// TODO: Implement in-place replace like sed -i -e, without the need for an intermediary file
func PatchFile(path string, search string, replace string) error {
	path = strings.TrimSpace(path) // Better safe than sorry
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	input, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	output := bytes.Replace(input, []byte(search), []byte(replace), -1)

	if err = ioutil.WriteFile(path+".patched", output, fi.Mode().Perm()); err != nil {
		return err
	}

	os.Rename(path+".patched", path)
	return nil
}
