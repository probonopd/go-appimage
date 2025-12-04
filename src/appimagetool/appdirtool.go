package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"strconv"
	"syscall"

	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otiai10/copy"
	"github.com/probonopd/go-appimage/internal/helpers"
)

type QMLImport struct {
	Classname    string `json:"classname,omitempty"`
	Name         string `json:"name"`
	Path         string `json:"path,omitempty"`
	Plugin       string `json:"plugin,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
	Type         string `json:"type"`
	Version      string `json:"version"`
}

var allELFs []string
var libraryLocations []string // All directories in the host system that may contain libraries
var seenDeps []string

var quirksModePatchQtPrfxPath = false

// normalizePathToUsrPrefix transforms paths outside of /usr to be under /usr
// to comply with the AppImage spec recommendation and FHS conventions.
// For example:
// - /lib/x86_64-linux-gnu/libfoo.so -> /usr/lib/x86_64-linux-gnu/libfoo.so
// - /lib64/libbar.so -> /usr/lib64/libbar.so
// - /usr/lib/libqux.so -> /usr/lib/libqux.so (unchanged)
// - /sbin/foo -> /usr/sbin/foo
// - /bin/bar -> /usr/bin/bar
// This ensures all deployed files follow the /usr prefix convention.
func normalizePathToUsrPrefix(path string) string {
	// Prefixes that should be moved under /usr
	prefixMappings := []struct {
		from string
		to   string
	}{
		{"/lib64/", "/usr/lib64/"},
		{"/lib/", "/usr/lib/"},
		{"/bin/", "/usr/bin/"},
		{"/sbin/", "/usr/sbin/"},
	}

	for _, mapping := range prefixMappings {
		if strings.HasPrefix(path, mapping.from) {
			return mapping.to + strings.TrimPrefix(path, mapping.from)
		}
	}

	return path
}

func getAppRunData() string {
	apprun := `#!/bin/sh

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

export PYTHONHOME="${HERE}"/usr/

############################################################################################
# Use bundled Tcl/Tk
############################################################################################

if [ -e "${HERE}"/usr/share/tcltk/tcl8.6 ] ; then
  export TCL_LIBRARY="${HERE}"/usr/share/tcltk/tcl8.6:$TCL_LIBRARY:$TK_LIBRARY
  export TK_LIBRARY="${HERE}"/usr/share/tcltk/tk8.6:$TK_LIBRARY:$TCL_LIBRARY
fi

############################################################################################
# Use the system OpenSSL certificates if available
############################################################################################

# On most modern machines
if [ -e /etc/ssl/certs/ca-certificates.crt ]; then
  export SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
# Legacy versions of RHEL/Fedora
elif [ -e /etc/ssl/certs/ca-bundle.crt ]; then
  export SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt
# Misc
elif [ -e /etc/ssl/cacert.pem ]; then
  export SSL_CERT_FILE=/etc/ssl/cacert.pem
elif [ -e /etc/ssl/cert.pem ]; then
  export SSL_CERT_FILE=/etc/ssl/cert.pem
fi

############################################################################################
# Make it look more native on Gtk+ based systems
############################################################################################

case "${XDG_CURRENT_DESKTOP}" in
    *GNOME*|*gnome*)
        export QT_QPA_PLATFORMTHEME=gtk2
esac`

	if options.preserveCwd == false {
		apprun += `

############################################################################################
# If .ui files are in the AppDir, then chances are that we need to cd into usr/
# because we may have had to patch the absolute paths away in the binary
############################################################################################

UIFILES=$(find "$HERE" -name "*.ui")
if [ ! -z "$UIFILES" ] ; then
  cd "$HERE/usr"
fi`
	}

	apprun += `

############################################################################################
# Use bundled GStreamer
# NOTE: May need to remove libgstvaapi.so
############################################################################################

if [ ! -z $(find "${HERE}" -name "libgstcoreelements.so" -type f) ] ; then
  export GST_PLUGIN_PATH=$(dirname $(readlink -f $(find "${HERE}" -name "libgstcoreelements.so" -type f | head -n 1)))
  export GST_PLUGIN_SCANNER=$(find "${HERE}" -name "gst-plugin-scanner" -type f | head -n 1)
  export GST_PLUGIN_SYSTEM_PATH=$GST_PLUGIN_PATH
  env | grep GST
fi

############################################################################################
# Run experimental bundle that bundles everything if a private ld-linux-x86-64.so.2 is there
# This allows the bundle to run even on older systems than the one it was built on
############################################################################################`

	if options.preserveCwd == false {
		apprun += `

cd "$HERE/usr"`
	}

	apprun += `

# Try to find a binary with the same name as the AppImage or the symlink through which
# it was invoked, without any suffix
if [ -z "$ARGV0" ] ; then
  # AppRun is being executed outside of an AppImage
  ARGV0="$0"
fi
BINARY_NAME=$(basename "$ARGV0")
if [ "$BINARY_NAME" = "AppRun" ] ; then
  unset BINARY_NAME
fi

BINARY_NAME="${BINARY_NAME%.*}" # remove everything after the last "."
MAIN_BIN=$(find "$HERE/usr/bin" -name "$BINARY_NAME" | head -n 1)

# Fall back to finding the main binary based on the Exec= line in the desktop file
if [ -z "$MAIN_BIN" ] ; then
  MAIN_BIN=$(find "$HERE/usr/bin" -name "$MAIN" | head -n 1)
fi

LD_LINUX=$(find "$HERE" -name 'ld-*.so.*' | head -n 1)

if [ -e "$LD_LINUX" ] ; then
  export GCONV_PATH="$HERE/usr/lib/gconv"
  export FONTCONFIG_FILE="$HERE/etc/fonts/fonts.conf"
  export GTK_PATH=$(find "$HERE/lib" -name gtk-* -type d)
  export GTK_THEME=Default # This one should be bundled so that it can work on systems without Gtk
  export GDK_PIXBUF_MODULEDIR=$(find "$HERE" -name loaders -type d -path '*gdk-pixbuf*')
  export GDK_PIXBUF_MODULE_FILE=$(find "$HERE" -name loaders.cache -type f -path '*gdk-pixbuf*') # Patched to contain no paths
  export XDG_DATA_DIRS="${HERE}"/usr/share/:"${XDG_DATA_DIRS}"
  export PERLLIB="${HERE}"/usr/share/perl5/:"${HERE}"/usr/lib/perl5/:"${PERLLIB}"
  export GSETTINGS_SCHEMA_DIR="${HERE}"/usr/share/glib-2.0/runtime-schemas/:"${HERE}"/usr/share/glib-2.0/schemas/:"${GSETTINGS_SCHEMA_DIR}"
  export QT_PLUGIN_PATH="$(readlink -f "$(dirname "$(find "${HERE}" -type d -path '*/plugins/platforms' 2>/dev/null)" 2>/dev/null)" 2>/dev/null)"
  case $line in
    "ld-linux"*) exec "${LD_LINUX}" --inhibit-cache "${MAIN_BIN}" "$@" ;;
    *) exec "${LD_LINUX}" "${MAIN_BIN}" "$@" ;;
  esac
else
  exec "${MAIN_BIN}" "$@"
fi
`
	return apprun
}

type ELF struct {
	path     string
	relpaths []string
	rpath    string
}

// Key: name of the package, value: location of the copyright file
var copyrightFiles = make(map[string]string) // Need to use 'make', otherwise we can't add to it

// Key: Path of the file, value: name of the package
var packagesContainingFiles = make(map[string]string) // Need to use 'make', otherwise we can't add to it

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

type DeployOptions struct {
	standalone     bool
	libAppRunHooks bool
	preserveCwd    bool
}

// this is the public options instance
// which need to be set before the function is called
var options DeployOptions

func AppDirDeploy(path string) {
	appdir, err := helpers.NewAppDir(path)
	if err != nil {
		helpers.PrintError("AppDir", err)
		os.Exit(1)
	}

	log.Println("Gathering all required libraries for the AppDir...")
	determineELFsInDirTree(appdir, appdir.Path)

	// Gdk
	handleGdk(appdir)

	// GStreamer
	handleGStreamer(appdir)

	// Gtk modules/plugins
	// If there is a .so with the name libgtk-* inside the AppDir, then we need to
	// bundle Gdk modules/plugins
	deployGtkDirectory(appdir, 4)
	deployGtkDirectory(appdir, 3)
	deployGtkDirectory(appdir, 2)

	deployGtkUiFiles(appdir)

	// ALSA
	handleAlsa(appdir)

	// PulseAudio
	handlePulseAudio(appdir)

	// ld-linux interpreter
	ldLinux, err := deployInterpreter(appdir)

	// Glib 2 schemas
	if helpers.Exists(appdir.Path + "/usr/share/glib-2.0/schemas") {
		err = handleGlibSchemas(appdir)
		if err != nil {
			helpers.PrintError("Could not deploy GLib schemas", err)
		}
	}
	// Fonts
	err = deployFontconfig(appdir)
	if err != nil {
		helpers.PrintError("Could not deploy Fontconfig", err)
	}

	// AppRun
	if options.libAppRunHooks == false {
		// If libapprun_hooks is not used
		log.Println("Adding AppRun...")
		err = os.WriteFile(appdir.Path+"/AppRun", []byte(getAppRunData()), 0755)
		if err != nil {
			helpers.PrintError("write AppRun", err)
			os.Exit(1)
		}
	} else {
		log.Println("TODO: Add AppRun suitable for libapprun_hooks...")
	}

	log.Println("Find out whether Qt is a dependency of the application to be bundled...")

	qtVersionDetected := 0

	for i := 5; i <= 99; i++ {
		if containsString(allELFs, fmt.Sprintf("libQt%dCore.so.%d", i, i)) == true {
			log.Printf("Detected Qt %d", i)
			qtVersionDetected = i
			break
		}
	}

	if containsString(allELFs, "libQtCore.so.4") == true {
		log.Println("Detected Qt 4")
		qtVersionDetected = 4
	}

	if qtVersionDetected > 0 {
		handleQt(appdir, qtVersionDetected)
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
			// Normalize the path to use /usr prefix per AppImage spec/FHS recommendation
			lib = appdir.Path + normalizePathToUsrPrefix(lib)
		}
		libraryLocationsInAppDir = helpers.AppendIfMissing(libraryLocationsInAppDir, lib)
	}
	fmt.Println("")

	log.Println("libraryLocationsInAppDir:")
	for _, lib := range libraryLocationsInAppDir {
		fmt.Println(lib)
	}
	fmt.Println("")

	/*
		fmt.Println("")
		log.Println("allELFs:")
		for _, lib := range allELFs {
			fmt.Println(lib)
		}
	*/

	log.Println("Only after this point should we start copying around any ELFs")

	log.Println("Copying in and patching ELFs which are not already in the AppDir...")

	handleNvidia()

	for _, lib := range allELFs {

		deployElf(lib, appdir, err)
		patchRpathsInElf(appdir, libraryLocationsInAppDir, lib)

		if strings.Contains(lib, fmt.Sprintf("libQt%dCore.so.%d", qtVersionDetected, qtVersionDetected)) {
			fmt.Println("Patching Qt prefix path in " + lib)
			patchQtPrfxpath(appdir, lib, libraryLocationsInAppDir, ldLinux)
		}
	}

	deployCopyrightFiles(appdir)
}

func deployFontconfig(appdir helpers.AppDir) error {
	var err error
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
	return err
}

func deployInterpreter(appdir helpers.AppDir) (string, error) {
	var ldLinux, err = appdir.GetElfInterpreter(appdir)
	if err != nil {
		helpers.PrintError("Could not determine ELF interpreter", err)
		os.Exit(1)
	}
	if helpers.Exists(appdir.Path+"/"+ldLinux) == true {
		log.Println("Removing pre-existing", ldLinux+"...")
		err = syscall.Unlink(appdir.Path + "/" + ldLinux)
		if err != nil {
			helpers.PrintError("Could not remove pre-existing ld-linux", err)
			os.Exit(1)
		}

	}
	if options.standalone {
		var err error
		// ld-linux might be a symlink; hence we first need to resolve it
		src, err := filepath.EvalSymlinks(ldLinux)
		if err != nil {
			helpers.PrintError("Could not get the location of ld-linux", err)
			src = ldLinux
		}

		log.Println("Deploying", ldLinux+"...")

		// Normalize the path to use /usr prefix per AppImage spec/FHS recommendation
		normalizedLdLinux := normalizePathToUsrPrefix(ldLinux)
		ldTargetPath := appdir.Path + normalizedLdLinux
		if options.libAppRunHooks {
			// This file is part of the libc family of libraries and we want to use libapprun_hooks,
			// hence copy to a separate directory unlike the rest of the libraries. The reason is
			// that this familiy of libraries will only be used by libapprun_hooks if the
			// bundled version is newer than what is already on the target system; this allows
			// us to also load libraries from the system such as proprietary GPU drivers
			log.Println(ldLinux, "is part of libc; copy to", LibcDir, "subdirectory")
			ldTargetPath = appdir.Path + "/" + LibcDir + normalizedLdLinux // If libapprun_hooks is used
		}
		err = copy.Copy(src, ldTargetPath)
		if err != nil {
			helpers.PrintError("Could not copy ld-linux", err)
			return "", err
		}
		// Do what we do in the Scribus AppImage script, namely
		// sed -i -e 's|/usr|/xxx|g' lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
		log.Println("Patching ld-linux...")
		err = PatchFile(ldTargetPath, "/lib", "/XXX")
		if err != nil {
			helpers.PrintError("PatchFile", err)
			return "", err
		}
		err = PatchFile(ldTargetPath, "/usr", "/xxx")
		if err != nil {
			helpers.PrintError("PatchFile", err)
			return "", err
		}
		// --inhibit-cache is not working, it is still using /etc/ld.so.cache
		err = PatchFile(ldTargetPath, "/etc", "/EEE")
		if err != nil {
			helpers.PrintError("PatchFile", err)
			return "", err
		}
		log.Println("Determining gconv (for GCONV_PATH)...")
		// Search in all of the system's library directories for a directory called gconv
		// and put it into the a location which matches the GCONV_PATH we export in AppRun
		gconvs, err := findWithPrefixInLibraryLocations("gconv")
		if err == nil {
			// Target location must match GCONV_PATH exported in AppRun
			determineELFsInDirTree(appdir, gconvs[0])
		}

		if err != nil {
			helpers.PrintError("Could not deploy the interpreter", err) // TODO: Don't print this for musl libc
			// os.Exit(1) // There is no gconf is musl libc
		}

		// Make ld-linux executable
		err = os.Chmod(ldTargetPath, 0755)
		if err != nil {
			helpers.PrintError("Could not set permissions on the interpreter", err)
			os.Exit(1)
		}
	} else {
		log.Println("Not deploying", ldLinux, "because it was not requested or it is not needed")
	}
	return ldLinux, err
}

// deployElf deploys an ELF (executable or shared library) to the AppDir
// if it is not on the exclude list and it is not yet at the target location
func deployElf(lib string, appdir helpers.AppDir, err error) {
	for _, excludePrefix := range ExcludedLibraries {
		if strings.HasPrefix(filepath.Base(lib), excludePrefix) == true && !options.standalone {
			log.Println("Skipping", lib, "because it is on the excludelist")
			return
		}
	}

	log.Println("Working on", lib)
	if strings.HasPrefix(lib, appdir.Path) == false { // Do not copy if it is already in the AppDir
		// Normalize the path to use /usr prefix per AppImage spec/FHS recommendation
		normalizedLib := normalizePathToUsrPrefix(lib)
		libTargetPath := appdir.Path + normalizedLib
		if options.libAppRunHooks && checkWhetherPartOfLibc(lib) == true {
			// This file is part of the libc family of libraries and we want to use libapprun_hooks,
			// hence copy to a separate directory unlike the rest of the libraries. The reason is
			// that this familiy of libraries will only be used by libapprun_hooks if the
			// bundled version is newer than what is already on the target system; this allows
			// us to also load libraries from the system such as proprietary GPU drivers
			log.Println(lib, "is part of libc; copy to", LibcDir, "subdirectory")
			libTargetPath = appdir.Path + "/" + LibcDir + normalizedLib // If libapprun_hooks is used
		}

		// Skip copying if the source is a directory
		if fi, err := os.Stat(lib); err == nil && fi.IsDir() {
			log.Println(lib, "is a directory, skipping")
			return
		}

		log.Println("Copying to libTargetPath:", libTargetPath)

		err = helpers.CopyFile(lib, libTargetPath) // If libapprun_hooks is not used

		if err != nil {
			log.Println(libTargetPath, "could not be copied:", err)
			// os.Exit(1)
		}
	}
}

// patchQtPrfxpath patches qt_prfxpath of the libQt5Core.so.5/libQt6Core.so.6 in an AppDir
// so that the Qt installation finds its own components in the AppDir
func patchQtPrfxpath(appdir helpers.AppDir, lib string, libraryLocationsInAppDir []string, ldLinux string) {
	log.Println("Patching qt_prfxpath, otherwise can't load platform plugin...")

	libPath := lib
	// Determine libPath:
	// if lib is inside the AppDir, use that; otherwise,
	// if appdir.Path + "/" + lib exists, use that. IMPORTANT: Use the absolute, not relative paths!
	absoluteAppDirPath, err := filepath.Abs(appdir.Path)
	if err != nil {
		helpers.PrintError("Could not determine absolute path of AppDir", err)
		os.Exit(1)
	}
	absoluteLibPath, err := filepath.Abs(lib)
	if err != nil {
		helpers.PrintError("Could not determine absolute path of lib", err)
		os.Exit(1)
	}
	if strings.HasPrefix(absoluteLibPath, absoluteAppDirPath) {
		libPath = absoluteLibPath
	} else {
		if _, err := os.Stat(absoluteAppDirPath + "/" + lib); err == nil {
			libPath = absoluteAppDirPath + "/" + lib
		} else {
			helpers.PrintError("Could not determine absolute path of lib", err)
			os.Exit(1)
		}
	}
	log.Println("libPath:", libPath)

	f, err := os.Open(libPath)
	// Get the filename of lib withouth the path, e.g., libQt5Core.so.5/libQt6Core.so.6
	libFilename := filepath.Base(lib)
	// Open file for reading/determining the offset
	defer f.Close()
	if err != nil {
		helpers.PrintError("Could not open "+libFilename+" for reading", err)
		os.Exit(1)
	}
	f.Seek(0, 0)
	// Search from the beginning of the file
	search := []byte("qt_prfxpath=")
	prfxpathPos := ScanFile(f, search)
	if prfxpathPos < 0 {
		helpers.PrintError("Could not find offset for " + string(search), errors.New("no " + string(search) + " token in binary"))
		os.Exit(1)
	}
	offset := prfxpathPos + int64(len(search))
	log.Println("Offset of qt_prfxpath:", offset)
	/*
		What does qt_prfxpath=. actually mean on a Linux system? Where is "."?
		Looks like it means "relative to args[0]".

		So 'qt_prfxpath=.' would be wrong if we load the application through ld-linux.so because that one
		lives in, e.g., appdir/lib64 which is actually not where the Qt 'plugins' directory is.
		Hence we do NOT have to patch qt_prfxpath=. but to qt_prfxpath=../opt/qt512/'
		(the relative path from args[0] to the directory in which the Qt 'plugins' directory is)
	*/
	// Note: The following is correct only if we bundle (and run through) ld-linux; in all other cases
	// we should calculate the relative path relative to the main binary
	var qtPrefixDir string
	for _, libraryLocationInAppDir := range libraryLocationsInAppDir {
		if strings.HasSuffix(libraryLocationInAppDir, "/plugins/platforms") {
			qtPrefixDir = filepath.Dir(filepath.Dir(libraryLocationInAppDir))
			break
		}
	}
	if qtPrefixDir == "" {
		helpers.PrintError("Could not determine the the Qt prefix directory:", err)
		os.Exit(1)
	} else {
		log.Println("Qt prefix directory in the AppDir:", qtPrefixDir)
	}
	relPathToQt, err := filepath.Rel(filepath.Dir(appdir.Path+ldLinux), qtPrefixDir)
	if err != nil {
		helpers.PrintError("Could not compute the location of the Qt plugins directory:", err)
		os.Exit(1)
	} else {
		log.Println("Relative path from ld-linux to Qt prefix directory in the AppDir:", relPathToQt)
	}
	f, err = os.OpenFile(libPath, os.O_WRONLY, 0644)
	// Open file writable, why is this so complicated
	defer f.Close()
	if err != nil {
		helpers.PrintError("Could not open "+libPath+" for writing", err)
		os.Exit(1)
	}
	// Now that we know where in the file the information is, go write it
	f.Seek(offset, 0)
	if quirksModePatchQtPrfxPath == false {
		log.Println("Patching qt_prfxpath in " + libPath + " to " + relPathToQt)
		_, err = f.Write([]byte(relPathToQt + "\x00"))
	} else {
		log.Println("Patching qt_prfxpath in " + libPath + " to ..")
		_, err = f.Write([]byte(".." + "\x00"))
	}
	if err != nil {
		helpers.PrintError("Could not patch qt_prfxpath in "+libPath, err)
	}
}

// deployCopyrightFiles deploys copyright files into the AppDir
// for each ELF in allELFs that are inside the AppDir and have matching equivalents outside of the AppDir
func deployCopyrightFiles(appdir helpers.AppDir) {
	log.Println("Copying in copyright files...")
	for _, lib := range allELFs {

		shouldDoIt := true
		for _, excludePrefix := range ExcludedLibraries {
			if strings.HasPrefix(filepath.Base(lib), excludePrefix) == true && options.standalone == false {
				log.Println("Skipping copyright file for ", lib, "because it is on the excludelist")
				shouldDoIt = false
				break
			}
		}

		if shouldDoIt == true && strings.HasPrefix(lib, appdir.Path) == false {
			// Copy copyright files into the AppImage
			copyrightFile, err := getCopyrightFile(lib)
			// It is perfectly fine for this to error - on non-dpkg systems, or if lib was not in a deb package
			if err == nil {
				os.MkdirAll(filepath.Dir(appdir.Path+copyrightFile), 0755)
				copy.Copy(copyrightFile, appdir.Path+copyrightFile)
			}

		}
	}
	log.Println("Done")
	if options.standalone == true {
		log.Println("To check whether it is really self-contained, run:")
		fmt.Println("LD_LIBRARY_PATH='' find " + appdir.Path + " -type f -exec ldd {} 2>&1 \\; | grep '=>' | grep -v " + appdir.Path)
	}

	if options.libAppRunHooks == true {
		log.Println("The option '-m' was used. Hence, you need to manually add AppRun, .env, and libapprun_hooks.so")
		log.Println("from https://github.com/AppImageCrafters/AppRun/releases/tag/continuous. TODO: Automate this")
	}

}

// handleGlibSchemas compiles GLib schemas if the subdirectory is present in the AppImage.
// AppRun has to export GSETTINGS_SCHEMA_DIR for this to work
func handleGlibSchemas(appdir helpers.AppDir) error {
	var err error
	if helpers.Exists(appdir.Path+"/usr/share/glib-2.0/schemas") && !helpers.Exists(appdir.Path+"/usr/share/glib-2.0/schemas/gschemas.compiled") {
		log.Println("Compiling glib-2.0 schemas...")
		cmd := exec.Command("glib-compile-schemas", ".")
		cmd.Dir = appdir.Path + "/usr/share/glib-2.0/schemas"
		err = cmd.Run()
		if err != nil {
			helpers.PrintError("Run glib-compile-schemas", err)
			os.Exit(1)
		}
	}
	return err
}

func handleGdk(appdir helpers.AppDir) {
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

					loadersCache, err := filepath.EvalSymlinks(filepath.Dir(loadersCaches[0]))
					if err != nil {
						helpers.PrintError("Could not get the location of loaders.cache", err)
						break
					}

					whatToPatchAway := loadersCache + "/loaders/"

					log.Println("Patching", appdir.Path+loadersCaches[0], "removing", whatToPatchAway)
					err = PatchFile(appdir.Path+loadersCaches[0], whatToPatchAway, "")
					if err != nil {
						helpers.PrintError("PatchFile loaders.cache", err)
						break // os.Exit(1)
					}

				}
			}
			break
		}
	}
}

func handlePulseAudio(appdir helpers.AppDir) {
	// TODO: What about the `/usr/lib/pulse-*` directory?
	for _, lib := range allELFs {
		if strings.HasPrefix(filepath.Base(lib), "libpulse.so") {
			log.Println("Bundling pulseaudio directory (for <tbd>)...")
			locs, err := findWithPrefixInLibraryLocations("pulseaudio")
			if err != nil {
				log.Println("Could not find pulseaudio directory")
				os.Exit(1)
			} else {
				log.Println("Bundling dependencies of pulseaudio directory...")
				determineELFsInDirTree(appdir, locs[0])
			}

			break
		}
	}
}

func handleNvidia() {
	// As soon as we bundle libnvidia*, we get a segfault.
	// Hence we exit whenever libGL.so.1 requires libnvidia*
	for _, elf := range allELFs {
		if strings.HasPrefix(filepath.Base(elf), "libnvidia") {
			log.Println("System (most likely libGL) uses libnvidia*, please build on another system that does not use NVIDIA drivers, exiting")
			os.Exit(1)
		}
	}
}

func handleAlsa(appdir helpers.AppDir) {
	// FIXME: Doesn't seem to get loaded. Is ALSA_PLUGIN_DIR needed and working in ALSA?
	// Is something like https://github.com/flatpak/freedesktop-sdk-images/blob/1.6/alsa-lib-plugin-path.patch needed in the bundled ALSA?
	// TODO: What about the `share/alsa` subdirectory? libasound.so.* refers to it as well
	for _, lib := range allELFs {
		if strings.HasPrefix(filepath.Base(lib), "libasound.so") {
			log.Println("Bundling alsa-lib directory (for <tbd>)...")
			locs, err := findWithPrefixInLibraryLocations("alsa-lib")
			if err != nil {
				log.Println("Could not find alsa-lib directory")
				log.Println("E.g., in Alpine Linux: apk add alsa-plugins alsa-plugins-pulse")
			} else {
				log.Println("Bundling dependencies of alsa-lib directory...")
				determineELFsInDirTree(appdir, locs[0])
			}

			break
		}
	}
}

func handleGStreamer(appdir helpers.AppDir) {
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

			break
		}
	}
}

func patchRpathsInElf(appdir helpers.AppDir, libraryLocationsInAppDir []string, path string) {

	if strings.HasPrefix(path, appdir.Path) == false {
		// Normalize the path to use /usr prefix per AppImage spec/FHS recommendation
		path = filepath.Clean(appdir.Path + normalizePathToUsrPrefix(path))
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

	if options.libAppRunHooks && checkWhetherPartOfLibc(path) {
		log.Println("Not writing rpath because file is part of the libc family of libraries")
		return
	}

	// If the file starts with ld- or libc., we don't patch the rpath
	// NOTE: In Alpine Linux, ld-musl-x86_64.so.1 links to libc.musl-x86_64.so.1
	basePath := filepath.Base(path)
	if strings.HasPrefix(basePath, "ld-") || strings.HasPrefix(basePath, "libc.") {
		log.Println("Not patching rpath because file starts with ld- or libc.")
	} else {
		// Call patchelf to set the rpath
		if helpers.Exists(path) == true {
			// log.Println("Rewriting rpath of", path)
			cmd := exec.Command("patchelf", "--set-rpath", newRpathStringForElf, path)
			// log.Println(cmd.Args)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println(cmd.String())
				helpers.PrintError("patchelf --set-rpath "+path+": "+string(out), err)
				os.Exit(1)
			}
		}
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

					if gtkVersion <= 3 {
						log.Println("Bundling Default theme for Gtk", strconv.Itoa(gtkVersion), "(for GTK_THEME=Default)...")
						err = copy.Copy("/usr/share/themes/Default/gtk-"+strconv.Itoa(gtkVersion)+".0", appdir.Path+"/usr/share/themes/Default/gtk-"+strconv.Itoa(gtkVersion)+".0")
						if err != nil {
							helpers.PrintError("Copy", err)
							os.Exit(1)
						}
					}

					if gtkVersion <= 3 {
						log.Println("Bundling immodules.cache for Gtk", strconv.Itoa(gtkVersion))
						immodulesCaches := helpers.FilesWithSuffixInDirectoryRecursive(loc, "immodules.cache")
						if len(immodulesCaches) < 1 {
							log.Println("Couldn't find immodules.cache")
							os.Exit(1)
						}
						immodulesCache := immodulesCaches[0]

						err = copy.Copy(immodulesCache, appdir.Path + immodulesCache)
						if err != nil {
							helpers.PrintError("Copy", err)
							os.Exit(1)
						}

						immodulesCacheLoc, err := filepath.EvalSymlinks(filepath.Dir(immodulesCache))
						if err != nil {
							helpers.PrintError("Could not get the location of immodules.cache", err)
							os.Exit(1)
						}

						whatToPatchAway := immodulesCacheLoc + "/immodules/"
						log.Println("Patching", appdir.Path + immodulesCache, "removing", whatToPatchAway)
						err = PatchFile(appdir.Path + immodulesCache, whatToPatchAway, "")
						if err != nil {
							helpers.PrintError("PatchFile immodules.cache", err)
							os.Exit(1)
						}
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

func deployGtkUiFiles(appdir helpers.AppDir) {
	// Check for the presence of Gtk .ui files
	uifiles := helpers.FilesWithSuffixInDirectoryRecursive(appdir.Path, ".ui")
	if len(uifiles) > 0 {
		log.Println("Gtk .ui files found. Need to take care to have them loaded from a relative rather than absolute path")
		log.Println("TODO: Check if they are at hardcoded absolute paths in the application and if yes, patch")
		var dirswithUiFiles []string
		for _, uifile := range uifiles {
			dirswithUiFiles = helpers.AppendIfMissing(dirswithUiFiles, filepath.Dir(uifile))
			err := PatchFile(appdir.MainExecutable, "/usr", "././")
			if err != nil {
				helpers.PrintError("PatchFile", err)
				os.Exit(1)
			}
		}
		log.Println("Directories with .ui files:", dirswithUiFiles)
	}
}

// appendLib appends library in path to allELFs and adds its location as well as any pre-existing rpaths to libraryLocations
func appendLib(path string) {

	for _, excludedlib := range ExcludedLibraries {
		if filepath.Base(path) == excludedlib && !options.standalone {
			// log.Println("Skipping", excludedlib, "because it is on the excludelist")
			return
		}
	}

	// Find out whether there are pre-existing rpaths and if so, add them to libraryLocations
	// so that we can find libraries there, too
	// See if the library had a pre-existing rpath that did not start with $. If so, replace it by one that
	// points to the equal location as the original but inside the AppDir
	rpaths, err := readRpaths(path)
	if err != nil {
		helpers.PrintError("Could not determine rpath in "+path, err)
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
		if err != nil {
			helpers.PrintError("getDeps", err)
			os.Exit(1)
		}
	}
	log.Println("len(allELFsUnderPath):", len(allELFsUnderPath))

	// Find out in which directories we now actually have libraries
	log.Println("libraryLocations:", libraryLocations)
	log.Println("len(allELFs):", len(allELFs))
}

func readRpaths(path string) ([]string, error) {
	// Call patchelf to find out whether the ELF already has an rpath set
	cmd := exec.Command("patchelf", "--print-rpath", path)
	// log.Println("patchelf cmd.Args:", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(cmd.String())
		helpers.PrintError("patchelf --print-rpath "+path+": "+string(out), err)
		log.Println("Perhaps it is not dynamically linked, or perhaps it is a script. Continuing...")
		// os.Exit(1)
		return []string{}, nil
	}
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
func findAllExecutablesAndLibraries(path string) ([]string, error) {
	var allExecutablesAndLibraries []string

	// fmt.Println(" findAllExecutablesAndLibrarieschecking", path)

	// If we have a file, then there is nothing to walk and we can return it directly
	if helpers.IsDirectory(path) != true {
		allExecutablesAndLibraries = append(allExecutablesAndLibraries, path)
		return allExecutablesAndLibraries, nil
	}

	filepath.Walk(path, func(path string, info os.FileInfo, e error) error {
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
	if containsString(seenDeps, binaryOrLib) == true {
		log.Println("skipping already seen dep, circular ref", binaryOrLib)
		return nil
	}
	var libs []string

	if helpers.IsDirectory(binaryOrLib) == true {
		return nil
	}

	if helpers.Exists(binaryOrLib) == false {
		return nil
	}

	e, err := elf.Open(binaryOrLib)
	// log.Println("getDeps", binaryOrLib)
	helpers.PrintError("elf.Open", err)

	// ImportedLibraries returns the names of all libraries
	// referred to by the binary f that are expected to be
	// linked with the binary at dynamic link time.
	libs, err = e.ImportedLibraries()
	helpers.PrintError("e.ImportedLibraries", err)

	err = e.Close()
	helpers.PrintError("e.Close", err)

	seenDeps = helpers.AppendIfMissing(seenDeps, binaryOrLib)
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

func isBlank(c rune) bool {
	return c == ' ' || c == '\t'
}

// getDirsFromSoConf returns a []string with the directories specified
// in the ld config file at path, usually '/etc/ld.so.conf',
// and in its included config files. We need to search in those locations
// for libraries as well
func getDirsFromSoConf(path string) []string {
	var out []string
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		} else if strings.HasPrefix(line, "include") && isBlank(([]rune(line))[7]) {
			incs := strings.FieldsFunc(line[8:], isBlank)
			for _, p := range incs {
				if p[0] != '/' {
					p = filepath.Dir(path) + "/" + p
				}
				files, err := filepath.Glob(p)
				if err != nil {
					return out
				}
				for _, file := range files {
					out = append(out, getDirsFromSoConf(file)...)
				}
			}
			continue
		} else if strings.HasPrefix(line, "hwcap") && isBlank(([]rune(line))[5]) {
			// Ignore hwcap directive, it's also ignore by glibc
			continue
		}
		out = append(out, strings.TrimSpace(line))
	}
	return out
}

func findLibrary(filename string) (string, error) {

	// Look for libraries in commonly used default locations
	locs := []string{"/usr/lib64", "/lib64", "/usr/lib", "/lib",
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

	// Additionally, look for libraries in the same locations in which glibc ld.so looks for libraries
	if helpers.Exists("/etc/ld.so.conf") {
		locs := getDirsFromSoConf("/etc/ld.so.conf")
		for _, loc := range locs {
			libraryLocations = helpers.AppendIfMissing(libraryLocations, filepath.Clean(loc))
		}
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

	input, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	output := bytes.Replace(input, []byte(search), []byte(replace), -1)

	if err = os.WriteFile(path+".patched", output, fi.Mode().Perm()); err != nil {
		return err
	}

	os.Rename(path+".patched", path)
	return nil
}

func getCopyrightFile(path string) (string, error) {

	var copyrightFile string

	if helpers.IsCommandAvailable("dpkg") == false {
		return copyrightFile, errors.New("dpkg not found, hence not deploying copyright files")
	}

	if helpers.IsCommandAvailable("dpkg-query") == false {
		return copyrightFile, errors.New("dpkg-query not found, hence not deploying copyright files")
	}

	// Find out which package the file being deployed belongs to
	var packageContainingTheSO string
	pkg, ok := packagesContainingFiles[path]
	if ok == true {
		packageContainingTheSO = pkg
	} else {
		cmd := exec.Command("dpkg", "-S", path)
		// log.Println("Find out which package the file being deployed belongs to using", cmd.String())
		result, err := cmd.Output()
		if err != nil {
			return copyrightFile, err
		}
		parts := strings.Split(strings.TrimSpace(string(result)), ":")
		packageContainingTheSO = parts[0]
	}

	// Find out the copyright file in that package
	// We are caching the results so that multiple packages belonging to the same package have to run dpkg-query only once
	// So first we check whether we already know it
	cf, ok := copyrightFiles[packageContainingTheSO]
	if ok == true {
		return cf, nil
	}

	cmd := exec.Command("dpkg-query", "-L", packageContainingTheSO)
	// log.Println("Find out the copyright file in that package using", cmd.String())
	output, err := cmd.Output()
	if err != nil {
		return copyrightFile, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		packagesContainingFiles[strings.TrimSpace(line)] = packageContainingTheSO
		if strings.Contains(line, "usr/share/doc") && strings.Contains(line, "copyright") {
			copyrightFile = strings.TrimSpace(line)
		}
	}
	if copyrightFile == "" {
		return copyrightFile, errors.New("could not determine the copyright file")
	} else {
		// log.Println("Copyright file:", copyrightFile)
		copyrightFiles[packageContainingTheSO] = copyrightFile
	}

	return copyrightFile, nil
}

// Let's see in how many lines of code we can re-implement the guts of linuxdeployqt
func handleQt(appdir helpers.AppDir, qtVersion int) {

	if qtVersion >= 5 {

		// Actually the libQt5Core.so.5/libQt6Core.so.6 contains (always?) qt_prfxpath=... which tells us the location in which 'plugins/' is located

		library, err := findLibrary(fmt.Sprintf("libQt%dCore.so.%d", qtVersion, qtVersion))
		if err != nil {
			helpers.PrintError(fmt.Sprintf("Could not find libQt%dCore.so.%d", qtVersion, qtVersion), err)
			os.Exit(1)
		}

		f, err := os.Open(library)
		defer f.Close()
		if err != nil {
			helpers.PrintError(fmt.Sprintf("Could not open libQt%dCore.so.%d", qtVersion, qtVersion), err)
			os.Exit(1)
		}

		qtPrfxpath := getQtPrfxpath(f, err, qtVersion)

		if qtPrfxpath == "" {
			log.Println("Got empty qtPrfxpath, exiting")
			os.Exit(1)
		}

		log.Println("Looking in", qtPrfxpath+"/plugins")

		if helpers.Exists(qtPrfxpath+"/plugins/platforms/libqxcb.so") == false {
			log.Println("Could not find 'plugins/platforms/libqxcb.so' in qtPrfxpath, exiting")
			os.Exit(1)
		}

		determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/platforms/libqxcb.so")

		// From here on, mark for deployment certain Qt components if certain conditions are true
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1250
		log.Println("Selecting for deployment required Qt plugins...")

		// GTK Theme, if it exists
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1244
		wants := []string{"libqgtk2.so", "libqgtk2style.so"}
		for _, want := range wants {
			found := helpers.FilesWithSuffixInDirectoryRecursive(qtPrfxpath, want)
			if len(found) > 0 {
				determineELFsInDirTree(appdir, found[0])
			}
		}

		// iconengines and imageformats, if libQt5Gui.so.5/libQt6Gui.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1259
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dGui.so.%d", qtVersion, qtVersion)) == true {
				if helpers.Exists(qtPrfxpath + "/plugins/iconengines/") {
					determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/iconengines/")
				} else {
					fmt.Println("Skipping", appdir, qtPrfxpath+"/plugins/iconengines/", "because it does not exist")
				}
				if helpers.Exists(qtPrfxpath + "/plugins/imageformats/") {
					determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/imageformats/")
				} else {
					fmt.Println("Skipping", appdir, qtPrfxpath+"/plugins/imageformats/", "because it does not exist")
				}
				break
			}
		}

        // platform plugin context - required for special characters - included if libQt5Gui.so.5/libQt6.Gui.so.6 is included
        // similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1229
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dGui.so.%d", qtVersion, qtVersion)) == true {
				if helpers.Exists(qtPrfxpath + "/plugins/platforminputcontexts/") {
					determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/platforminputcontexts/")
				} else {
					fmt.Println("Skipping", appdir, qtPrfxpath+"/plugins/platforminputcontexts/", "because it does not exist")
				}
				break
			}
		}

		// Platform OpenGL context, if one of several libraries is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1282
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dGui.so.%d", qtVersion, qtVersion)) == true ||
				strings.HasSuffix(lib, fmt.Sprintf("libQt%dOpenGL.so.%d", qtVersion, qtVersion)) == true ||
				strings.HasSuffix(lib, fmt.Sprintf("libQt%dXcbQpa.so.%d", qtVersion, qtVersion)) == true ||
				strings.HasSuffix(lib, "libxcb-glx.so") == true {
				{
					determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/xcbglintegrations/")
					break
				}
			}
		}

		// CUPS print support plugin, if libQt5PrintSupport.so.5/libQt6PrintSupport.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1299
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dPrintSupport.so.%d", qtVersion, qtVersion)) == true {
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/printsupport/libcupsprintersupport.so")
				break
			}
		}

		// Network bearers, if libQt5Network.so.5/libQt6Network.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1304
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dNetwork.so.%d", qtVersion, qtVersion)) == true {
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/bearer/")
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/tls/")

				// TLS plugins in Qt 6 require OpenSSL 3
				if qtVersion >= 6 {
					sslLibrary, err := findLibrary("libssl.so.3")
					if err != nil {
						helpers.PrintError("Could not find libssl.so.3", err)
						os.Exit(1)
					}
					determineELFsInDirTree(appdir, sslLibrary)
				}

				break
			}
		}

		// Sql drivers, if libQt5Sql.so.5/libQt6Sql.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1312
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dSql.so.%d", qtVersion, qtVersion)) == true {
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/sqldrivers/")
				break
			}
		}

		// Positioning plugins, if libQt5Positioning.so.5/libQt6Positioning.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1320
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dPositioning.so.%d", qtVersion, qtVersion)) == true {
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/position/")
				break
			}
		}

		// Multimedia plugins, if libQt5Multimedia.so.5/libQt6Multimedia.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1328
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dMultimedia.so.%d", qtVersion, qtVersion)) == true {
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/mediaservice/")
				determineELFsInDirTree(appdir, qtPrfxpath+"/plugins/audio/")
				break
			}
		}

		// WebEngine, if libQt5WebEngineCore.so.5/libQt6WebEngineCore.so.6 is about to be deployed
		// similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1343
		for _, lib := range allELFs {
			if strings.HasSuffix(lib, fmt.Sprintf("libQt%dWebEngineCore.so.%d", qtVersion, qtVersion)) == true {
				log.Println("TODO: Deploying Qt5WebEngine components...")
				os.Exit(1)

				wants := []string{"QtWebEngineProcess",
					"qtwebengine_resources.pak",
					"qtwebengine_devtools_resources.pak",
					"qtwebengine_resources_100p.pak",
					"qtwebengine_resources_200p.pak",
					"icudtl.dat",
					"qtwebengine_locales"}
				for _, want := range wants {
					found := helpers.FilesWithSuffixInDirectoryRecursive(qtPrfxpath, want)
					if len(found) > 0 {
						err = os.MkdirAll(filepath.Dir(appdir.Path+"/"+found[0]), 0755)
						if err != nil {
							helpers.PrintError("could not create directory", err)
							os.Exit(1)
						}
						err = copy.Copy(found[0], appdir.Path+"/"+found[0]) // TODO: Test. Not tested yet
						if err != nil {
							helpers.PrintError("could not copy file or directory", err)
							os.Exit(1)
						}
					}
				}
			}
		}

		// eploy QML
		// Similar to https://github.com/probonopd/linuxdeployqt/blob/42e51ea7c7a572a0aa1a21fc47d0f80032809d9d/tools/linuxdeployqt/shared.cpp#L1541
		log.Println("TODO: Deploying QML components...")

		qmlImportScanners := helpers.FilesWithSuffixInDirectoryRecursive(qtPrfxpath, "qmlimportscanner")
		if len(qmlImportScanners) < 1 {
			log.Println("qmlimportscanner not found, skipping QML deployment") // TODO: Exit if we have qml files and qmlimportscanner is not there
			return
		} else {
			log.Println("Found qmlimportscanner:", qmlImportScanners[0])
		}
		qmlImportScanner := qmlImportScanners[0]

		// Locate the qml directory, usually it is directly within the Qt prefix directory
		// FIXME: Maybe a more elaborate logic for locating this is needed
		importPath := qtPrfxpath + "/qml"

		log.Println("Deploying QML imports ")
		log.Println("Application QML file path(s) is " + filepath.Dir(appdir.MainExecutable))
		log.Println("QML module search path(s) is " + importPath)
		log.Println("TODO: Allow for users to supply additional '-importPath' paths")
		// https://ilyabiz.com/2018/11/automatic-qml-import-by-qt-deployment-tools/
		// PRs welcome

		// Run qmlimportscanner
		cmd := exec.Command(qmlImportScanner, "-rootPath", filepath.Dir(appdir.Path), "-importPath", importPath)
		out, err := cmd.Output()
		if err != nil {
			fmt.Println(cmd.String())
			helpers.PrintError("qmlscanner: "+string(out), err)
			os.Exit(1)
		}

		// Parse the JSON from qmlimportscanner
		// "If you have data whose structure or property names you are not certain of,
		// you cannot use structs to unmarshal your data. Instead you can use maps"
		// https://www.sohamkamani.com/blog/2017/10/18/parsing-json-in-golang/
		// "To deal with this case we create a map of strings to empty interfaces:"
		// var data map[string]interface{}
		// The above gives
		// panic: json: cannot unmarshal array into Go value of type map[string]interface {}
		// so we are using this, which apparently works:
		// var data []map[string]interface{}
		// if err := json.Unmarshal(out, &data); err != nil {
		//	panic(err)
		// }

		var qmlImports []QMLImport
		if err := json.Unmarshal(out, &qmlImports); err != nil {
			panic(err)
		}

		fmt.Println(qmlImports)

		for _, qmlImport := range qmlImports {

			if qmlImport.Type == "module" && qmlImport.Path != "" {
				log.Println("qmlImport.Type:", qmlImport.Type)
				log.Println("qmlImport.Name:", qmlImport.Name)
				log.Println("qmlImport.Path:", qmlImport.Path)
				log.Println("qmlImport.RelativePath:", qmlImport.RelativePath)
				os.MkdirAll(filepath.Dir(path.Join(appdir.Path, qmlImport.Path)), 0755)
				copy.Copy(qmlImport.Path, path.Join(appdir.Path, qmlImport.Path)) // FIXME: Ideally we would not copy here but only after the point where we start copying everything
				path.Join("sss", "sss")
				determineELFsInDirTree(appdir, path.Join(appdir.Path, qmlImport.Path))
			}
		}
	}
}

func getQtPrfxpath(f *os.File, err error, qtVersion int) string {

	// If the user has set $QTDIR or $QT_ROOT_DIR, use that instead of the one from qt_prfxpath in the library
	qtPrefixEnv := os.Getenv("QTDIR")
	if qtPrefixEnv == "" {
	    qtPrefixEnv = os.Getenv("QT_ROOT_DIR")
	}
	if qtPrefixEnv != "" {
	    log.Println("Using QTDIR or QT_ROOT_DIR:", qtPrefixEnv)
	    return qtPrefixEnv
	}

	f.Seek(0, 0)
	// Search from the beginning of the file
	search := []byte("qt_prfxpath=")
	offset := ScanFile(f, search) + int64(len(search))
	log.Println("Offset of qt_prfxpath:", offset)
	search = []byte("\x00")
	// From the current location in the file, search to the next 0x00 byte
	f.Seek(offset, 0)
	length := ScanFile(f, search)
	log.Println("Length of value of qt_prfxpath:", length)
	// Now that we know where in the file the information is, go get it
	f.Seek(offset, 0)
	buf := make([]byte, length)
	// Make a buffer that is exactly as long as the range we want to read
	_, err = io.ReadFull(f, buf)
	if err != nil {
		helpers.PrintError("Unable to read qt_prfxpath", err)
		os.Exit(1)
	}
	qt_prfxpath := strings.TrimSpace(string(buf))
	log.Println("qt_prfxpath:", qt_prfxpath)
	if qt_prfxpath == "" {
		log.Println("Could not get qt_prfxpath")
		return ""
	}

	// Special case:
	// Some distributions, including Ubuntu and Alpine,
	// have qt_prfxpath set to '/usr' but the files are actually in e.g., '/usr/lib/qt5'
	// In this case, we should NOT patch it
	if helpers.IsDirectory(qt_prfxpath+"/plugins") == false {
		log.Println("Got qt_prfxpath but it does not contain 'plugins'")
		results := helpers.FilesWithSuffixInDirectoryRecursive(qt_prfxpath, "libqxcb.so")
		log.Println("libqxcb.so found:", results)
		if len(results) > 0 {
			result := results[0]
			qt_prfxpath = filepath.Dir(filepath.Dir(filepath.Dir(result)))
			log.Println("Guessed qt_prfxpath to be", qt_prfxpath)
			quirksModePatchQtPrfxPath = true
		} else {
			log.Println("ERROR: Could not determine the path to Qt automatically")
			log.Println("Please set $QTDIR to the path to Qt (the directory that contains plugins/, qml/, etc.)")
			log.Println("and try again")
			os.Exit(1)
		}
	}

	return qt_prfxpath
}

// ScanFile returns the offset of the first occurrence of a []byte in a file from the current position,
// or -1 if []byte was not found in file, and seeks to the beginning of the searched []byte
// https://forum.golangbridge.org/t/how-to-find-the-offset-of-a-byte-in-a-large-binary-file/16457/
func ScanFile(f io.ReadSeeker, search []byte) int64 {
	ix := 0
	r := bufio.NewReader(f)
	offset := int64(0)
	for ix < len(search) {
		b, err := r.ReadByte()
		if err != nil {
			return -1
		}
		if search[ix] == b {
			ix++
		} else {
			ix = 0
		}
		offset++
	}
	f.Seek(offset-int64(len(search)), 0) // Seeks to the beginning of the searched []byte
	return offset - int64(len(search))
}

// checkWhetherPartOfLibc returns true if the file passed in belongs to one of the libc6,
// zlib1g, libstdc++6 packages, otherwise returns false. This is needed because libapprun_hooks
// uses those only if the libc on the system is older than the libc in the AppDir, so that
// system libraries like those needed for Nvidia GPU acceleration can still be loaded.
// Rather than using the system's package manager, this is using a hardcoded list of filenames
// and paths which may need to be adjusted over time. This is to be distribution-independent.
func checkWhetherPartOfLibc(thisfile string) bool {

	prefixes := []string{"ld", "libBrokenLocale", "libSegFault", "libanl", "libc", "libdl", "libm", "libmemusage", "libmvec", "libnsl",
		"libnss", "libpcprofile", "libpthread", "libresolv", "librt", "libthread_db", "libutil", "sotruss-lib", "libz", "libstdc++"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(filepath.Base(thisfile), prefix+"-") || strings.HasPrefix(filepath.Base(thisfile), prefix+".") || strings.HasPrefix(filepath.Base(thisfile), prefix+"_") {
			return true
		}
	}

	pathparts := []string{"/ld.so.conf.d/", "/libstdcxx/", "/gconv/"}

	for _, pathpart := range pathparts {
		if strings.Contains(thisfile, pathpart) {
			return true
		}
	}

	return false
}
