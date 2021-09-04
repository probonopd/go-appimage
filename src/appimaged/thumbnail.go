package main

import (
	"bufio"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	_ "embed"

	"github.com/adrg/xdg"
	issvg "github.com/h2non/go-is-svg"
	"github.com/probonopd/go-appimage/internal/helpers"
	pngembed "github.com/sabhiram/png-embed" // For embedding metadata into PNG
	"github.com/srwiley/oksvg"               // https://github.com/niemeyer/gopkg/issues/72
	"github.com/srwiley/rasterx"
)

/* The thumbnail cache directory is prefixed with $XDG_CACHE_DIR/ and the leading dot removed
(since $XDG_CACHE_DIR is normally $HOME/.cache).
The glib ChangeLog indicates the path for large sizes was "fixed" (Added $XDG_CACHE_DIR) starting with 2.35.3 */
var ThumbnailsDirNormal = xdg.CacheHome + "/thumbnails/normal/"

func (ai AppImage) extractDirIconAsThumbnail() {
	// log.Println("thumbnail: extract DirIcon as thumbnail")
	if ai.Type() <= 0 {
		return
	}

	// TODO: Detect Modifications by reading the 'Thumb::MTime' key as per
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS

	// Write out the icon to a temporary location
	thumbnailcachedir := xdg.CacheHome + "/thumbnails/" + ai.md5
	os.MkdirAll(thumbnailcachedir, os.ModePerm)

	// if ai.imagetype == 1 {
	// 	err := os.MkdirAll(thumbnailcachedir, os.ModePerm)
	// 	helpers.LogError("thumbnail: thumbnailcachedir", err)
	// 	cmd := exec.Command("bsdtar", "-C", thumbnailcachedir, "-xf", ai.path, ".DirIcon")
	// 	runCommand(cmd)
	// } else if ai.imagetype == 2 {
	// 	// TODO: first list contents of the squashfs, then determine what to extract
	// 	cmd := exec.Command("unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", thumbnailcachedir, ai.path, ".DirIcon")
	// 	runCommand(cmd)
	// }

	//this will try to extract the thumbnail, or goes back to command based extraction if it fails.
	var dirIconFil *os.File
	dirIconRdr, err := ai.Thumbnail()
	if err != nil {
		if *verbosePtr {
			log.Print("Could not find .DirIcon, trying to find the desktop file's specified icon")
		}
		dirIconRdr, _, err = ai.Icon()
		if err != nil {
			goto genericIcon
		}
	}
	dirIconFil, _ = os.Create(thumbnailcachedir + "/.DirIcon")
	_, err = io.Copy(dirIconFil, dirIconRdr)
	dirIconRdr.Close()
	dirIconFil.Close()
	if err != nil {
		helpers.LogError("thumbnail", err)
	}
	//TODO: I could probably dump it directly to the buffer below
	// if err != nil {
	// Too verbose
	// sendErrorDesktopNotification(ai.niceName+" may be defective", "Could not read .DirIcon")
	// }
genericIcon:
	buf, err := ioutil.ReadFile(thumbnailcachedir + "/.DirIcon")
	if os.IsNotExist(err) {
		if *verbosePtr {
			log.Printf("Could not extract icon, use default icon instead: %s\n", thumbnailcachedir+"/.DirIcon")
		}
		//go:embed embed/appimage.png
		var defaultIcon []byte
		err = os.MkdirAll(thumbnailcachedir, 0755)
		helpers.LogError("thumbnail", err)
		err = ioutil.WriteFile(thumbnailcachedir+"/.DirIcon", defaultIcon, 0644)
		helpers.LogError("thumbnail", err)
	} else if err != nil {
		log.Printf("Error: %s\n", err)
	}
	if issvg.Is(buf) {
		log.Println("thumbnail: .DirIcon in", ai.Path, "is an SVG, this is discouraged. Costly converting it now")
		err = convertToPng(thumbnailcachedir + "/.DirIcon")
		helpers.LogError("thumbnail", err)
	}

	// Before we proceed, delete empty files. Otherwise the following operations can crash
	// TODO: Better check if it is a PNG indeed
	fi, err := os.Stat(thumbnailcachedir + "/.DirIcon")
	helpers.LogError("thumbnail", err)
	if err != nil {
		return
	} else if fi.Size() == 0 {
		err = os.Remove(thumbnailcachedir + "/.DirIcon")
		helpers.LogError("thumbnail", err)
		return
	}

	f, _ := os.Open(thumbnailcachedir + "/.DirIcon")
	defer f.Close()

	if !helpers.CheckMagicAtOffset(f, "504e47", 1) {
		log.Println("thumbnail: Not a PNG file, hence removing:", thumbnailcachedir+"/.DirIcon")
		err = os.Remove(thumbnailcachedir + "/.DirIcon")
		helpers.LogError("thumbnail", err)
	}

	// Write "Thumbnail Attributes" metadata as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#ADDINFOS
	// and set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// Thumb::URI	The absolute canonical uri for the original file. (eg file:///home/jens/photo/me.jpg)

	// FIXME; github.com/sabhiram/png-embed does not overwrite pre-existing values,
	// https://github.com/sabhiram/png-embed/issues/1

	content, err := pngembed.ExtractFile(thumbnailcachedir + "/.DirIcon")

	if *verbosePtr {
		if _, ok := content["Thumb::URI"]; ok {
			log.Println("thumbnail: FIXME: Remove pre-existing Thumb::URI in", ai.Path)
			// log.Println(content["Thumb::URI"])
		}
		if _, ok := content["Thumb::MTime"]; ok {
			log.Println("thumbnail: FIXME: Remove pre-existing Thumb::MTime", content["Thumb::MTime"], "in", ai.Path) // FIXME; pngembed does not seem to overwrite pre-existing values, is it a bug there?
			// log.Println(content["Thumb::MTime"])
		}
	}
	helpers.LogError("thumbnail "+thumbnailcachedir+"/.DirIcon", err)
	data, err := pngembed.EmbedFile(thumbnailcachedir+"/.DirIcon", "Thumb::URI", ai.uri)
	helpers.LogError("thumbnail", err)

	/* Set 'Thumb::MTime' metadata of the thumbnail file to the mtime of the AppImage.
	NOTE: https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS says:
	It is not sufficient to do a file.mtime > thumb.MTime check.
	If the user moves another file over the original, where the mtime changes but is in fact lower
	than the thumbnail stored mtime, we won't recognize this modification.
	If for some reason the thumbnail doesn't have the 'Thumb::MTime' key (although it's required)
	it should be recreated in any case. */
	if appImageInfo, e := os.Stat(ai.Path); e == nil {
		_, e = pngembed.EmbedFile(thumbnailcachedir+"/.DirIcon", "Thumb::MTime", appImageInfo.ModTime())
		helpers.LogError("thumbnail", e)
	}

	if err == nil {
		err = ioutil.WriteFile(thumbnailcachedir+"/.DirIcon", data, 0600)
		helpers.LogError("thumbnail", err)
	}

	// Set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// err = os.Chmod(thumbnailcachedir+"/.DirIcon", 0600)
	// printError("thumbnail", err)

	// After all the processing is done, move the icons to their real location
	// where they are (hopefully) picked up by the desktop environment
	err = os.MkdirAll(ThumbnailsDirNormal, os.ModePerm)
	helpers.LogError("thumbnail", err)

	if *verbosePtr {
		log.Println("thumbnail: Moving", thumbnailcachedir+"/.DirIcon", "to", ai.thumbnailfilepath)
	}

	err = os.Rename(thumbnailcachedir+"/.DirIcon", ai.thumbnailfilepath)
	helpers.LogError("thumbnail", err)

	/* Also set mtime of the thumbnail file to the mtime of the AppImage. Quite possibly this is not needed.
	TODO: Perhaps we can remove it.
	See https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS  */
	if appImageInfo, e := os.Stat(ai.Path); e == nil {
		e = os.Chtimes(ai.thumbnailfilepath, time.Now().Local(), appImageInfo.ModTime())
		helpers.LogError("thumbnail", e)
	}

	err = os.RemoveAll(thumbnailcachedir)
	helpers.LogError("thumbnail", err)

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
	icon, err := oksvg.ReadIcon(filePath, oksvg.WarnErrorMode)
	if err != nil {
		return err
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scannerGV := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scannerGV)
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
