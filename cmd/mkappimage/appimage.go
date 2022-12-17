package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/probonopd/go-appimage/internal/helpers"
)

// Handles AppImage files.
// Currently it is using using a static build of mksquashfs/unsquashfs
// but eventually may be rewritten to do things natively in Go

type AppImage struct {
	path      string
	imagetype int
	offset    int64
}

// NewAppImage creates an AppImage object from the location defined by path.
// The AppImage object will also be created if path does not exist,
// because the AppImage that used to be there may need to be removed
// and for this the functions of an AppImage are needed.
// Non-existing and invalid AppImages will have type -1.
func NewAppImage(path string) AppImage {

	ai := AppImage{path: path, imagetype: 0}

	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		ai.imagetype = -1
		return ai
	}
	ai.imagetype = ai.DetermineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.imagetype < 0 {
		return ai
	}
	if ai.imagetype < 1 {
		return ai
	}
	if ai.imagetype > 1 {
		ai.offset = helpers.CalculateElfSize(ai.path)
	}
	// ai.discoverContents() // Only do when really needed since this is slow
	// log.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXX rawcontents:", ai.rawcontents)
	// Besides, for whatever reason it is not working properly yet

	return ai
}

// Fills rawcontents with the raw output of our extraction tools,
// libarchive and unsquashfs. This is a slow operation and should hence only be done
// once we are sure that we really need this information.
// Maybe we should consider to have a fixed directory inside the AppDir
// for everything that should be extracted, or a MANIFEST file. That would save
// us this slow work at runtime
func (ai AppImage) ShowContents(isLong bool) error {
	// Let's get the listing of files inside the AppImage. We can work on this later on
	// to resolve symlinks, and to determine which files to extract in addition to the desktop file and icon
	cmd := exec.Command("")
	if ai.imagetype == 1 {

		cmd = exec.Command("bsdtar", "-t", ai.path)
	} else if ai.imagetype == 2 {
		listCommand := "-l"
		if isLong {
			listCommand = "-ll"
		}
		// cmd = exec.Command("unsquashfs", "-h")
		cmd = exec.Command("unsquashfs", "-f", "-n", listCommand, "-o", strconv.FormatInt(ai.offset, 10), ai.path)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

// Check whether we have an AppImage at all.
// Return image type, or -1 if it is not an AppImage
func (ai AppImage) DetermineImageType() int {
	// log.Println("appimage: ", ai.path)
	f, err := os.Open(ai.path)

	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}

	info, err := os.Stat(ai.path)
	if err != nil {
		return -1
	}

	// Directories cannot be AppImages, so return fast
	if info.IsDir() {
		return -1
	}

	// Very small files cannot be AppImages, so return fast
	if info.Size() < 100*1024 {
		return -1
	}

	if helpers.CheckMagicAtOffset(f, "414902", 8) {
		return 2
	}

	if helpers.CheckMagicAtOffset(f, "414901", 8) {
		return 1
	}

	// ISO9660 files that are also ELF files
	if helpers.CheckMagicAtOffset(f, "7f454c", 0) && helpers.CheckMagicAtOffset(f, "4344303031", 32769) {
		return 1
	}

	return -1
}
