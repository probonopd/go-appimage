package main

import (
	"bufio"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/adrg/xdg"
	issvg "github.com/h2non/go-is-svg"
	"github.com/sabhiram/png-embed" // For embedding metadata into PNG
	. "github.com/srwiley/oksvg"
	. "github.com/srwiley/rasterx"
	ini "gopkg.in/ini.v1"
)

func (ai AppImage) extractDirIconAsThumbnail() {
	// log.Println("thumbnail: extract DirIcon as thumbnail")
	if ai.imagetype <= 0 {
		return
	}

	// TODO: Detect Modifications by reading the 'Thumb::MTime' key as per
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS

	// Write out the icon to a temporary location
	thumbnailcachedir := xdg.CacheHome + "/thumbnails/" + ai.md5

	if ai.imagetype == 1 {
		err := os.MkdirAll(thumbnailcachedir, os.ModePerm)
		printError("thumbnail: thumbnailcachedir", err)
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
		// log.Println(file.Name())
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
					printError("thumbnail: thumbnailcachedir", err)
					cmd := exec.Command(here()+"/unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, iconvalue)
					runCommand(cmd)
					err = os.Rename(thumbnailcachedir+"/"+iconvalue, thumbnailcachedir+"/.DirIcon")
					printError("thumbnail", err)
					err = os.RemoveAll(thumbnailcachedir + "/" + file.Name())
					printError("thumbnail", err)
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

	buf, err := ioutil.ReadFile(thumbnailcachedir + "/.DirIcon")
	if os.IsNotExist(err) {
		log.Printf("Could not extract icon, use default icon instead: %s\n", thumbnailcachedir+"/.DirIcon")
		data, err := Asset("data/appimage.png")
		printError("thumbnail", err)
		err = os.MkdirAll(thumbnailcachedir, 0755)
		printError("thumbnail", err)
		err = ioutil.WriteFile(thumbnailcachedir+"/.DirIcon", data, 0644)
		printError("thumbnail", err)
	} else if err != nil {
		log.Printf("Error: %s\n", err)
	}
	if issvg.Is(buf) {
		log.Println("thumbnail: .DirIcon in", ai.path, "is an SVG, this is discouraged. Costly converting it now")
		err = convertToPng(thumbnailcachedir + "/.DirIccon")
		printError("thumbnail", err)
	}

	// Before we proceed, delete empty files. Otherwise the following operations can crash
	// TODO: Better check if it is a PNG indeed
	fi, err := os.Stat(thumbnailcachedir + "/.DirIcon")
	printError("thumbnail", err)
	if err != nil {
		return
	} else if fi.Size() == 0 {
		os.Remove(thumbnailcachedir + "/.DirIcon")
		return
	}

	f, err := os.Open(thumbnailcachedir + "/.DirIcon")
	defer f.Close()

	if checkMagicAtOffset(f, "504e47", 1) == false {
		log.Println("thumbnail: Not a PNG file, hence removing:", thumbnailcachedir+"/.DirIcon")
		os.Remove(thumbnailcachedir + "/.DirIcon")
	}

	// Write "Thumbnail Attributes" metadata as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#ADDINFOS
	// and set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// Thumb::URI	The absolute canonical uri for the original file. (eg file:///home/jens/photo/me.jpg)

	content, err := pngembed.ExtractFile(thumbnailcachedir + "/.DirIcon")
	if _, ok := content["Thumb::URI"]; ok {
		log.Println("thumbnail: FIXME: Remove pre-existing Thumb::URI in", ai.path) // FIXME; github.com/sabhiram/png-embed does not seem to overwrite pre-existing values, is it a bug there?
		// log.Println(content["Thumb::URI"])
	}
	if _, ok := content["Thumb::MTime"]; ok {
		log.Println("thumbnail: FIXME: Remove pre-existing Thumb::MTime", content["Thumb::MTime"], "in", ai.path) // FIXME; pngembed does not seem to overwrite pre-existing values, is it a bug there?
		// log.Println(content["Thumb::MTime"])
	}
	printError("thumbnail "+thumbnailcachedir+"/.DirIcon", err)
	data, err := pngembed.EmbedFile(thumbnailcachedir+"/.DirIcon", "Thumb::URI", ai.uri)
	// TODO: Thumb::MTime
	printError("thumbnail", err)
	if err == nil {
		err = ioutil.WriteFile(thumbnailcachedir+"/.DirIcon", data, 600)
		printError("thumbnail", err)
	}

	// Set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// err = os.Chmod(thumbnailcachedir+"/.DirIcon", 0600)
	// printError("thumbnail", err)

	// After all the processing is done, move the icons to their real location
	// where they are (hopefully) picked up by the desktop environment
	home, _ := os.UserHomeDir()
	err = os.MkdirAll(home+"/.thumbnails/normal/", os.ModePerm)
	printError("thumbnail", err)
	log.Println("thumbnail: Moving", thumbnailcachedir+"/.DirIcon", "to", ai.thumbnailfilepath)
	err = os.Rename(thumbnailcachedir+"/.DirIcon", ai.thumbnailfilepath)
	printError("thumbnail", err)
	err = os.RemoveAll(thumbnailcachedir)
	printError("thumbnail", err)

	// In Xfce, the new thumbnail is not shown in the file manager until we touch the file
	// In fact, touching it from within this program makes the thumbnail not work at all
	// TODO: Read XDG Thumbnail spec regarding this
	// The following all does not work
	// time.Sleep(2 * time.Second)
	// now := time.Now()
	// err = os.Chtimes(ai.path, now, now)
	// printError("thumbnail", err)
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
