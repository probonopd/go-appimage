package main

import (
	"C"
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	issvg "github.com/h2non/go-is-svg"
	. "github.com/srwiley/oksvg"
	. "github.com/srwiley/rasterx"
	"gopkg.in/ini.v1"
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
	result := url.PathEscape(ai.path)
	result = strings.Replace(result, "%28", "(", -1)
	result = strings.Replace(result, "%29", ")", -1)
	result = strings.Replace(result, "%2F", "/", -1)
	result = "file://" + result
	// log.Println("-->", result)
	hasher := md5.New()
	hasher.Write([]byte(result))
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

func (ai AppImage) extractDirIconAsThumbnail() {
	log.Println("appimage: extract DirIcon as thumbnail")
	if ai.imagetype <= 0 {
		return
	}

	//cmd := exec.Command("")

	// Write out the icon to a temporary location
	thumbnailcachedir := xdg.CacheHome + "/thumbnails/" + ai.md5

	if ai.imagetype == 1 {
		err := os.MkdirAll(thumbnailcachedir, os.ModePerm)
		printError("appimage: thumbnailcachedir", err)
		cmd := exec.Command(here()+"/bsdtar", "-C", thumbnailcachedir, "-xf", ai.path, ".DirIcon")
		runCommand(cmd)
	} else if ai.imagetype == 2 {
		// TODO: first list contents of the squashfs, then determine what to extract
		cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, ".DirIcon")
		runCommand(cmd)
	}

	// What we have just extracted may well have been a symlink
	// hence we try to resolve it
	fileInfo, err := ioutil.ReadDir(thumbnailcachedir)
	for _, file := range fileInfo {
		log.Println(file.Name())
		originFile, err := os.Readlink(thumbnailcachedir + "/" + file.Name())
		// If we could resolve the symlink, then extract its parent
		// and throw the symlink away
		if err == nil {
			if ai.imagetype == 1 {
				log.Println("TODO: Not yet implemented for type-1: We have a symlink, extract the original file")
			} else if ai.imagetype == 2 {
				cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, originFile)
				runCommand(cmd)
			}
			err = os.RemoveAll(thumbnailcachedir + "/.DirIcon")                              // Remove the symlink
			err = os.Rename(thumbnailcachedir+"/"+originFile, thumbnailcachedir+"/.DirIcon") // Put the real file there instead
			printError("appimage", err)
			// TODO: Rinse and repeat: May we still have a symlink at this point?
		}
	}

	// Workaround for electron-builder not generating .DirIcon
	// We may still not have an icon. For example, AppImages made by electron-builder
	// are lacking .DirIcon files as of Fall 2019; here we have to parse the desktop
	// file, and try to extract the value of Icon= with the suffix ".png" from the AppImage
	if Exists(thumbnailcachedir+"/.DirIcon") == false && ai.imagetype == 2 {
		log.Println(".DirIcon extraction failed. Is it missing? Trying to figure out alternative")
		cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, "*.desktop")
		runCommand(cmd)
		files, _ := ioutil.ReadDir(thumbnailcachedir)
		for _, file := range files {
			if filepath.Ext(thumbnailcachedir+file.Name()) == ".desktop" {
				log.Println("Determine iconname from desktop file:", thumbnailcachedir+"/"+file.Name())
				cfg, err := ini.Load(thumbnailcachedir + "/" + file.Name())
				if err == nil {
					section, _ := cfg.GetSection("Desktop Entry")
					iconkey, _ := section.GetKey("Icon")
					iconvalue := iconkey.Value() + ".png" // We are just assuming ".png" here
					log.Println("iconname from desktop file:", iconvalue)
					printError("appimage: thumbnailcachedir", err)
					cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, iconvalue)
					runCommand(cmd)
					err = os.Rename(thumbnailcachedir+"/"+iconvalue, thumbnailcachedir+"/.DirIcon")
					printError("appimage", err)
					err = os.RemoveAll(thumbnailcachedir + "/" + file.Name())
					printError("appimage", err)
				}
			}
		}

		// Workaround for electron-builder not generating .DirIcon
		// Also for the fallback:
		// What we have just extracted may well have been a symlink (in the case of electron-builder, it is)
		// hence we try to resolve it
		fileInfo, err = ioutil.ReadDir(thumbnailcachedir)
		for _, file := range fileInfo {
			log.Println(file.Name())
			originFile, err := os.Readlink(thumbnailcachedir + "/" + file.Name())
			// If we could resolve the symlink, then extract its parent
			// and throw the symlink away
			if err == nil {
				if ai.imagetype == 1 {
					log.Println("TODO: Not yet implemented for type-1: We have a symlink, extract the original file")
				} else if ai.imagetype == 2 {
					cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, originFile)
					runCommand(cmd)
				}
				err = os.RemoveAll(thumbnailcachedir + "/.DirIcon")                              // Remove the symlink
				err = os.Rename(thumbnailcachedir+"/"+originFile, thumbnailcachedir+"/.DirIcon") // Put the real file there instead
				printError("appimage", err)
				// TODO: Rinse and repeat: May we still have a symlink at this point?
			}
		}
	}

	// Check whether we have actually extracted an SVG and if so convert it

	// f, err := os.Open(thumbnailcachedir + "/.DirIcon")
	// printError("appimage", err)
	// if checkMagicAtOffset(f, "13780787113102610", 0) {
	// 	log.Println(thumbnailcachedir + "/.DirIcon is a PNG")
	// }
	// f.Close()

	buf, err := ioutil.ReadFile(thumbnailcachedir + "/.DirIcon")
	if os.IsNotExist(err) {
		log.Printf("Could not extract icon, use default icon instead: %s\n", thumbnailcachedir+"/.DirIcon")
		data, err := Asset("data/appimage.png")
		printError("appimage", err)
		err = os.MkdirAll(thumbnailcachedir, 0755)
		printError("appimage", err)
		err = ioutil.WriteFile(thumbnailcachedir+"/.DirIcon", data, 0644)
		printError("appimage", err)
	} else if err != nil {
		log.Printf("Error: %s\n", err)
	}
	if issvg.Is(buf) {
		log.Println(thumbnailcachedir + "/.DirIcon is an SVG, this is discouraged")
		err = convertToPng(thumbnailcachedir + "/.DirIcon")
		printError("appimage", err)
	}

	home, _ := os.UserHomeDir()
	err = os.MkdirAll(home+"/.thumbnails/normal/", os.ModePerm)
	printError("appimage", err)
	err = os.Rename(thumbnailcachedir+"/.DirIcon", ai.thumbnailfilepath)
	printError("appimage", err)
	err = os.RemoveAll(thumbnailcachedir)
	printError("appimage", err)

	// In Xfce, the new thumbnail is not shown in the file manager until we touch the file
	// In fact, touching it from within this program makes the thumbnail not work at all
	// TODO: Read XDG Thumbnail spec regarding this
	// The following all does not work
	// time.Sleep(2 * time.Second)
	// now := time.Now()
	// err = os.Chtimes(ai.path, now, now)
	// printError("appimage", err)
	// cmd = exec.Command("touch", ai.thumbnailfilepath)
}

// Convert a given file into a PNG; its dependencies add about 2 MB to the executable
func convertToPng(filePath string) error {
	// Strange colors: https://github.com/srwiley/oksvg/issues/15
	icon, err := ReadIcon(filePath, WarnErrorMode)
	if err != nil {
		return err
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scannerGV := NewScannerGV(w, h, img, img.Bounds())
	raster := NewDasher(w, h, scannerGV)
	icon.Draw(raster, 1.0)
	err = saveToPngFile(filePath, img)
	if err != nil {
		return err
	}
	return nil
}

func saveToPngFile(filePath string, m image.Image) error {
	// Create the file
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	// Create Writer from file
	b := bufio.NewWriter(f)
	// Write the image into the buffer
	err = png.Encode(b, m)
	if err != nil {
		return err
	}
	err = b.Flush()
	if err != nil {
		return err
	}
	return nil
}

// Check whether we have an AppImage at all.
// Return image type, or -1 if it is not an AppImage
func (ai AppImage) determineImageType() int {
	log.Println(ai.path)
	f, err := os.Open(ai.path)
	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}
	if info, err := os.Stat(ai.path); err == nil && info.IsDir() {
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

	f.Close()
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
func (ai AppImage) integrate() {

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

	if *quietPtr == false {
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

func (ai AppImage) removeIntegration() {

	if _, err := os.Stat(ai.path); os.IsNotExist(err) {

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
			if *quietPtr == false {
				sendDesktopNotification(ai.path, "Integration removed")
			}
		} else {
			log.Println("appimage:", err, ai.desktopfilepath)
		}
	}
}

func ioReader(file string) io.ReaderAt {
	r, err := os.Open(file)
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
