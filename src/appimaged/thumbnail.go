package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
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

//go:embed embed/appimage.png
var defaultIcon []byte

//Tries to get .DirIcon or the desktop's icon (in that order). If a failure, return generic icon.
func (ai AppImage) getThumbnailOrIcon() (out []byte) {
	fallback := defaultIcon
	rdr, err := ai.Thumbnail()
	if err != nil {
		fmt.Println("Error getting thumbnail")
		goto icon
	}
	out, err = io.ReadAll(rdr)
	if err != nil {
		fmt.Println("Error reading thumbnail")
		goto icon
	}
	if issvg.Is(out) {
		fmt.Println("Thumbnail is svg, checking desktop icon")
		fallback = out
		goto icon
	}
	return
icon:
	fmt.Println("Checking icon")
	rdr, _, err = ai.Icon()
	if err != nil {
		fmt.Println("Error getting icon")
		return fallback
	}
	out, err = io.ReadAll(rdr)
	if err != nil {
		fmt.Println("Error reading icon")
		return fallback
	}
	return
}

//This reads the icon from the appimage then makes necessary changes or uses the default icon as necessary.
//All this is now done in memory instead of constantly writing the changes to disk.
func (ai AppImage) extractDirIconAsThumbnail() {
	// log.Println("thumbnail: extract DirIcon as thumbnail")
	if ai.Type() <= 0 {
		return
	}

	// TODO: Detect Modifications by reading the 'Thumb::MTime' key as per
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS
	var err error
	iconBuf := ai.getThumbnailOrIcon()
	if issvg.Is(iconBuf) {
		log.Println("thumbnail: .DirIcon in", ai.Path, "is an SVG, this is discouraged. Costly converting it now")
		iconBuf, err = convertToPng(iconBuf)
		if err != nil {
			helpers.LogError("thumbnail", err)
			iconBuf = defaultIcon
		}
	}

	if !helpers.CheckMagicAtOffsetBytes(iconBuf, "504e47", 1) {
		log.Println("thumbnail: Not a PNG file, using generic icon")
		iconBuf = defaultIcon
	}

	// Write "Thumbnail Attributes" metadata as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#ADDINFOS
	// and set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// Thumb::URI	The absolute canonical uri for the original file. (eg file:///home/jens/photo/me.jpg)

	// FIXME; github.com/sabhiram/png-embed does not overwrite pre-existing values,
	// https://github.com/sabhiram/png-embed/issues/1

	// content, err := pngembed.Extract(iconBuf)

	// if *verbosePtr {
	// 	if _, ok := content["Thumb::URI"]; ok {
	// 		log.Println("thumbnail: FIXME: Remove pre-existing Thumb::URI in", ai.Path)
	// 		// log.Println(content["Thumb::URI"])
	// 	}
	// 	if _, ok := content["Thumb::MTime"]; ok {
	// 		log.Println("thumbnail: FIXME: Remove pre-existing Thumb::MTime", content["Thumb::MTime"], "in", ai.Path) // FIXME; pngembed does not seem to overwrite pre-existing values, is it a bug there?
	// 		// log.Println(content["Thumb::MTime"])
	// 	}
	// }
	out, err := pngembed.Embed(iconBuf, "Thumb::URI", ai.uri)
	helpers.LogError("thumbnail", err)
	if err == nil {
		iconBuf = out
	}
	/* Set 'Thumb::MTime' metadata of the thumbnail file to the mtime of the AppImage.
	NOTE: https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS says:
	It is not sufficient to do a file.mtime > thumb.MTime check.
	If the user moves another file over the original, where the mtime changes but is in fact lower
	than the thumbnail stored mtime, we won't recognize this modification.
	If for some reason the thumbnail doesn't have the 'Thumb::MTime' key (although it's required)
	it should be recreated in any case. */
	if appImageInfo, e := os.Stat(ai.Path); e == nil {
		out, err = pngembed.Embed(iconBuf, "Thumb::MTime", appImageInfo.ModTime())
		if err != nil {
			helpers.LogError("thumbnail", err)
		} else {
			iconBuf = out
		}
	}
	helpers.LogError("thumbnail", err)

	// Set thumbnail permissions to 0600 as mandated by
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#AEN245
	// err = os.Chmod(thumbnailcachedir+"/.DirIcon", 0600)
	// printError("thumbnail", err)

	err = os.MkdirAll(ThumbnailsDirNormal, os.ModePerm)
	helpers.LogError("thumbnail", err)

	if *verbosePtr {
		log.Println("thumbnail: Writing icon to", ai.thumbnailfilepath)
	}
	err = os.WriteFile(ai.thumbnailfilepath, iconBuf, 0600)
	helpers.LogError("thumbnail", err)

	/* Also set mtime of the thumbnail file to the mtime of the AppImage. Quite possibly this is not needed.
	TODO: Perhaps we can remove it.
	See https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS  */
	if appImageInfo, e := os.Stat(ai.Path); e == nil {
		e = os.Chtimes(ai.thumbnailfilepath, time.Now().Local(), appImageInfo.ModTime())
		helpers.LogError("thumbnail", e)
	}

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
func convertToPng(iconBuf []byte) ([]byte, error) {
	// Strange colors: https://github.com/srwiley/oksvg/issues/15
	icon, err := oksvg.ReadIconStream(bytes.NewReader(iconBuf), oksvg.WarnErrorMode)
	if err != nil {
		return nil, err
	}
	w, h := int(icon.ViewBox.W), int(icon.ViewBox.H)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scannerGV := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scannerGV)
	icon.Draw(raster, 1.0)
	buf := bytes.NewBuffer(make([]byte, 0))
	err = png.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
