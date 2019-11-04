package main

import (
	"C"

	"bytes"
	"crypto/md5"
	"encoding/hex"

	"io"

	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/go-language-server/uri"
)

// Handles AppImage files.
// Currently it is using using a static build of mksquashfs/unsquashfs
// but eventually may be rewritten to do things natively in Go

type AppImage struct {
	path              string
	imagetype         int
	uri               string
	md5               string
	desktopfilename   string
	desktopfilepath   string
	thumbnailfilename string
	thumbnailfilepath string
	offset            int64
	rawcontents       string
}

func newAppImage(path string) AppImage {

	ai := AppImage{path: path, imagetype: 0}

	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") {
		ai.imagetype = -1
		return ai
	}
	ai.uri = strings.TrimSpace(string(uri.File(filepath.Clean(ai.path))))
	ai.md5 = ai.calculateMD5filenamepart() // Need this also for non-existing AppImages for removal
	ai.desktopfilename = "appimagekit_" + ai.md5 + ".desktop"
	ai.desktopfilepath = xdg.DataHome + "/applications/" + "appimagekit_" + ai.md5 + ".desktop"
	ai.thumbnailfilename = ai.md5 + ".png"
	home, _ := os.UserHomeDir()
	ai.thumbnailfilepath = home + "/.thumbnails/normal/" + ai.thumbnailfilename
	ai.imagetype = ai.determineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.imagetype < 0 {
		return ai
	}
	ai.offset = calculateElfSize(ai.path)
	// ai.discoverContents() // Only do when really needed since this is slow
	return ai
}

// Fills rawcontents with the raw output of our extraction tools,
// libarchive and unsquashfs. This is a slow operation and should hence only be done
// once we are sure that we really need this information.
// Maybe we should consider to have a fixed directory inside the AppDir
// for everything that should be extracted, or a MANIFEST file. That would save
// us this slow work at runtime
func (ai AppImage) discoverContents() {
	// Let's get the listing of files inside the AppImage. We can work on this later on
	// to resolve symlinks, and to determine which files to extract in addition to the desktop file and icon
	cmd := exec.Command("")
	if ai.imagetype == 1 {
		cmd = exec.Command(here()+"/bsdtar", "-t", "'"+ai.path+"'")
	} else if ai.imagetype == 2 {
		cmd = exec.Command(here()+"/unsquashfs", "-f", "-n", "-ll", "-o", strconv.FormatInt(ai.offset, 10), "-d", "'"+ai.path+"'")
	}
	log.Printf("cmd: %q\n", cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	printError("appimage: list files:", err)
	ai.rawcontents = out.String()

}

func (ai AppImage) calculateMD5filenamepart() string {
	hasher := md5.New()
	hasher.Write([]byte(ai.uri))
	return hex.EncodeToString(hasher.Sum(nil))
}

func runCommand(cmd *exec.Cmd) (io.Writer, error) {

	log.Printf("runCommand: %q\n", cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	// printError("runCommand", err)
	// log.Println(cmd.Stdout)
	return cmd.Stdout, err
}

// Check whether we have an AppImage at all.
// Return image type, or -1 if it is not an AppImage
func (ai AppImage) determineImageType() int {
	// log.Println("appimage: ", ai.path)
	f, err := os.Open(ai.path)
	defer f.Close()

	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}

	info, err := os.Stat(ai.path)

	if err != nil {
		return -1
	}

	if err == nil && info.IsDir() {
		return -1
	}

	// Very small files cannot be AppImages, so return fast
	if err == nil && info.Size() < 10*1024 {
		return -1
	}

	if checkMagicAtOffset(f, "414902", 8) == true {
		return 2
	}

	if checkMagicAtOffset(f, "414901", 8) == true {
		return 1
	}

	// ISO9660 files that are also ELF files
	if checkMagicAtOffset(f, "7f454c", 0) == true && checkMagicAtOffset(f, "4344303031", 32769) == true {
		return 1
	}

	return -1
}

// Return true if magic string (hex) is found at offset
// TODO: Instead of magic string, could probably use something like []byte{'\r', '\n'} or []byte("AI")
func checkMagicAtOffset(f *os.File, magic string, offset int64) bool {
	_, err := f.Seek(offset, 0) // Go to offset
	printError("checkMagicAtOffset", err)
	b := make([]byte, len(magic)/2) // Read bytes
	n, err := f.Read(b)
	printError("checkMagicAtOffset", err)
	hexmagic := hex.EncodeToString(b[:n])
	if hexmagic == magic {
		log.Printf("checkMagicAtOffset: %v: Magic 0x%x at offset %v\n", f.Name(), string(b[:n]), offset)
		return true
	}
	return false
}

func (ai AppImage) setExecBit() {

	err := os.Chmod(ai.path, 0755)
	if err == nil {
		log.Println("appimage: Set executable bit on", ai.path)
	}
	// printError("appimage", err) // Do not print error since AppImages on read-only media are common
}

// Integrate an AppImage into the system (put in menu, extract thumbnail)
// Can take a long time, hence run with "go"
func (ai AppImage) _integrate() {

	// log.Println("integrate called on:", ai.path)

	// Return immediately if this is not an AppImage
	if ai.imagetype < 0 {
		// log.Println("Not an AppImage:", ai.path)
		return
	}

	ai.setExecBit()

	// For performance reasons, we stop working immediately
	// in case a desktop file already exists at that location
	if *overwritePtr == false {
		// Compare mtime of desktop file and AppImage, similar to
		// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS
		if desktopFileInfo, err := os.Stat(ai.desktopfilepath); err == nil {
			if appImageInfo, err := os.Stat(ai.path); err == nil {
				diff := desktopFileInfo.ModTime().Sub(appImageInfo.ModTime())
				if diff > (time.Duration(0) * time.Second) {
					// Do nothing if the desktop file is already newer than the AppImage file
					return
				}
			}
		}
	}

	writeDesktopFile(ai) // Do not run with "go" as it would interfere with extractDirIconAsThumbnail

	if *notifPtr == true {
		sendDesktopNotification(ai.path, "Integrated")
	}

	// For performance reasons, we stop working immediately
	// in case a thumbnail file already exists at that location
	if *overwritePtr == false {
		// Compare mtime of thumbnail file and AppImage, similar to
		// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS
		if thumbnailFileInfo, err := os.Stat(ai.thumbnailfilepath); err == nil {
			if appImageInfo, err := os.Stat(ai.path); err == nil {
				diff := thumbnailFileInfo.ModTime().Sub(appImageInfo.ModTime())
				if diff > (time.Duration(0) * time.Second) {
					// Do nothing if the thumbnail file is already newer than the AppImage file
					return
				}
			}
		}
	}

	ai.extractDirIconAsThumbnail() // Do not run with "go" as it would interfere with writeDesktopFile

}

func (ai AppImage) _removeIntegration() {

	log.Println("appimage: Remove integration", ai.path)
	err := os.Remove(ai.thumbnailfilepath)
	if err == nil {
		log.Println("appimage: Deleted", ai.thumbnailfilepath)
	} else {
		log.Println("appimage:", err, ai.thumbnailfilepath)
	}

	err = os.Remove(ai.desktopfilepath)
	if err == nil {
		log.Println("appimage: Deleted", ai.desktopfilepath)
		if *notifPtr == true {
			sendDesktopNotification(ai.path, "Integration removed")
		}

	}
}

func (ai AppImage) integrateOrUnintegrate() {
	if _, err := os.Stat(ai.path); os.IsNotExist(err) {
		ai._removeIntegration()
	} else {
		ai._integrate()
	}
}

func ioReader(file string) io.ReaderAt {
	r, err := os.Open(file)
	defer r.Close()
	printError("appimage: elf:", err)
	return r
}

// Returns true if file exists
func Exists(name string) bool {
	_, err := os.Stat(name)
	if err == nil {
		return true
	}
	return false
}
