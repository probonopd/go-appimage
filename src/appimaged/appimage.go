package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"net/url"

	"gopkg.in/ini.v1"

	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/probonopd/go-appimage/src/goappimage"
	"go.lsp.dev/uri"
)

// AppImage Handles AppImage files.
type AppImage struct {
	*goappimage.AppImage
	uri               string
	md5               string
	desktopfilename   string
	desktopfilepath   string
	thumbnailfilename string
	thumbnailfilepath string
	updateinformation string
	// offset            int64
	// rawcontents       string
	// niceName          string
}

// NewAppImage creates an AppImage object from the location defined by path.
// The AppImage object will also be created if path does not exist,
// because the AppImage that used to be there may need to be removed
// and for this the functions of an AppImage are needed.
// Non-existing and invalid AppImages will have type -1.
func NewAppImage(path string) (ai *AppImage, err error) {
	ai = new(AppImage)
	ai.AppImage, err = goappimage.NewAppImage(path)
	if err != nil {
		return ai, err
	}

	ai.uri = strings.TrimSpace(string(uri.File(filepath.Clean(ai.Path))))
	ai.md5 = ai.calculateMD5filenamepart() // Need this also for non-existing AppImages for removal
	ai.desktopfilename = "appimagekit_" + ai.md5 + ".desktop"
	ai.desktopfilepath = xdg.DataHome + "/applications/" + "appimagekit_" + ai.md5 + ".desktop"
	ai.thumbnailfilename = ai.md5 + ".png"
	if strings.HasSuffix(ThumbnailsDirNormal, "/") {
		ai.thumbnailfilepath = ThumbnailsDirNormal + ai.thumbnailfilename
	} else {
		ai.thumbnailfilepath = ThumbnailsDirNormal + "/" + ai.thumbnailfilename
	}
	ui, err := ai.ReadUpdateInformation()
	if err == nil && ui != "" {
		ai.updateinformation = ui
	}

	return ai, nil
}

func (ai AppImage) calculateMD5filenamepart() string {
	hasher := md5.New()
	hasher.Write([]byte(ai.uri))
	return hex.EncodeToString(hasher.Sum(nil))
}

// func (ai AppImage) calculateNiceName() string {
// 	niceName := filepath.Base(ai.Path())
// 	niceName = strings.Replace(niceName, ".AppImage", "", -1)
// 	niceName = strings.Replace(niceName, ".appimage", "", -1)
// 	niceName = strings.Replace(niceName, "-x86_64", "", -1)
// 	niceName = strings.Replace(niceName, "-i386", "", -1)
// 	niceName = strings.Replace(niceName, "-i686", "", -1)
// 	niceName = strings.Replace(niceName, "-", " ", -1)
// 	niceName = strings.Replace(niceName, "_", " ", -1)
// 	return niceName
// }

func runCommand(cmd *exec.Cmd) (bytes.Buffer, error) {
	if *verbosePtr == true {
		log.Printf("runCommand: %q\n", cmd)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	// printError("runCommand", err)
	// log.Println(cmd.Stdout)
	return out, err
}

func (ai AppImage) setExecBit() {

	err := os.Chmod(ai.Path, 0755)
	if err == nil {
		if *verbosePtr == true {
			log.Println("appimage: Set executable bit on", ai.Path)
		}
	}
	// printError("appimage", err) // Do not print error since AppImages on read-only media are common
}

// Validate checks the quality of an AppImage and sends desktop notification, returns error or nil
// TODO: Add more checks and reuse this in appimagetool
func (ai AppImage) Validate() error {
	if *verbosePtr == true {
		log.Println("Validating AppImage", ai.Path)
	}
	// Check validity of the updateinformation in this AppImage, if it contains some
	if ai.updateinformation != "" {
		log.Println("Validating updateinformation in", ai.Path)
		err := helpers.ValidateUpdateInformation(ai.updateinformation)
		if err != nil {
			helpers.PrintError("appimage: updateinformation verification", err)
			return err
		}
	}
	return nil
}

// Do not call this directly. Instead, call IntegrateOrUnintegrate
// Integrate an AppImage into the system (put in menu, extract thumbnail)
// Can take a long time, hence run with "go"
func (ai AppImage) _integrate() {

	// log.Println("integrate called on:", ai.path)

	// Return immediately if the filename extension is not .AppImage or .app
	if (strings.HasSuffix(ai.Path, ".AppImage") != true) && (strings.HasSuffix(ai.Path, ".app") != true) {
		// log.Println("No .AppImage suffix:", ai.path)
		return
	}

	ai.setExecBit()

	// For performance reasons, we stop working immediately
	// in case a desktop file already exists at that location
	if *overwritePtr == false {
		// Compare mtime of desktop file and AppImage, similar to
		// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS
		if desktopFileInfo, err := os.Stat(ai.desktopfilepath); err == nil {
			if appImageInfo, err := os.Stat(ai.Path); err == nil {
				diff := desktopFileInfo.ModTime().Sub(appImageInfo.ModTime())
				if diff > (time.Duration(0) * time.Second) {
					// Do nothing if the desktop file is already newer than the AppImage file
					// but subscribe
					if CheckIfConnectedToNetwork() == true {
						go SubscribeMQTT(MQTTclient, ai.updateinformation)
					}
					return
				}
			}
		}
	}

	// Let's be evil and integrate only good AppImages...
	// err := ai.Validate()
	// if err != nil {
	// 	log.Println("AppImage did not pass validation:", ai.path)
	// 	return
	// }

	writeDesktopFile(ai) // Do not run with "go" as it would interfere with extractDirIconAsThumbnail

	// Subscribe to MQTT messages for this application
	if ai.updateinformation != "" {
		if CheckIfConnectedToNetwork() == true {
			go SubscribeMQTT(MQTTclient, ai.updateinformation)
		}
	}

	// SimpleNotify(ai.path, "Integrated", 3000)

	// For performance reasons, we stop working immediately
	// in case a thumbnail file already exists at that location
	// if *overwritePtr == false {
	// Compare mtime of thumbnail file and AppImage, similar to
	// https://specifications.freedesktop.org/thumbnail-spec/thumbnail-spec-latest.html#MODIFICATIONS
	if thumbnailFileInfo, err := os.Stat(ai.thumbnailfilepath); err == nil {
		if appImageInfo, err := os.Stat(ai.Path); err == nil {
			diff := thumbnailFileInfo.ModTime().Sub(appImageInfo.ModTime())
			if diff > (time.Duration(0) * time.Second) {
				// Do nothing if the thumbnail file is already newer than the AppImage file
				return
			}
		}
	}
	// }

	ai.extractDirIconAsThumbnail() // Do not run with "go" as it would interfere with writeDesktopFile

}

// Do not call this directly. Instead, call IntegrateOrUnintegrate
func (ai AppImage) _removeIntegration() {
	log.Println("appimage: Remove integration", ai.Path)
	err := os.Remove(ai.thumbnailfilepath)
	if err == nil {
		log.Println("appimage: Deleted", ai.thumbnailfilepath)
	} else {
		log.Println("appimage:", err, ai.thumbnailfilepath)
	}

	// Unsubscribe to MQTT messages for this application
	if ai.updateinformation != "" {
		go UnSubscribeMQTT(MQTTclient, ai.updateinformation)
	}

	err = os.Remove(ai.desktopfilepath)
	if err == nil {
		log.Println("appimage: Deleted", ai.desktopfilepath)
		sendDesktopNotification("Removed", ai.Path, 3000)

	}
}

// IntegrateOrUnintegrate integrates or unintegrates
// (registers or unregisters) an AppImage from the system,
// depending on whether the file exists on disk. NEVER call this directly,
// ONLY have this called from a function that limits parallelism and ensures
// uniqueness of the AppImages to be processed
func (ai AppImage) IntegrateOrUnintegrate() {
	if _, err := os.Stat(ai.Path); os.IsNotExist(err) {
		ai._removeIntegration()
	} else {
		ai._integrate()
	}
}

// ReadUpdateInformation reads updateinformation from an AppImage
// Returns updateinformation string and error
func (ai AppImage) ReadUpdateInformation() (string, error) {
	aibytes, err := helpers.GetSectionData(ai.Path, ".upd_info")
	ui := strings.TrimSpace(string(bytes.Trim(aibytes, "\x00")))
	if err != nil {
		return "", err
	}
	// Don't validate here, we don't want to get warnings all the time.
	// We have AppImage.Validate as its own function which we call less frequently than this.
	return ui, nil
}

// LaunchMostRecentAppImage launches an the most recent application for a given
// updateinformation that we found among the integrated AppImages.
// Kinda like poor man's Launch Services. Probably we should make as much use of it as possible.
// Downside: Applications without updateinformation cannot be used in this way.
func LaunchMostRecentAppImage(updateinformation string, args []string) {
	if updateinformation == "" {
		return
	}
	if *quietPtr == false {
		aipath := FindMostRecentAppImageWithMatchingUpdateInformation(updateinformation)
		log.Println("Launching", aipath, args)
		cmd := []string{aipath}
		cmd = append(cmd, args...)
		err := helpers.RunCmdTransparently(cmd)
		if err != nil {
			helpers.PrintError("LaunchMostRecentAppImage", err)
		}

	}
}

// FindMostRecentAppImageWithMatchingUpdateInformation finds the most recent registered AppImage
// that havs matching upate information embedded
func FindMostRecentAppImageWithMatchingUpdateInformation(updateinformation string) string {
	results := FindAppImagesWithMatchingUpdateInformation(updateinformation)
	mostRecent := helpers.FindMostRecentFile(results)
	return mostRecent
}

// FindAppImagesWithMatchingUpdateInformation finds registered AppImages
// that have matching upate information embedded
func FindAppImagesWithMatchingUpdateInformation(updateinformation string) []string {
	files, err := ioutil.ReadDir(xdg.DataHome + "/applications/")
	helpers.LogError("desktop", err)
	var results []string
	if err != nil {
		return results
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") && strings.HasPrefix(file.Name(), "appimagekit_") {

			cfg, e := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
				xdg.DataHome+"/applications/"+file.Name())
			helpers.LogError("desktop", e)
			dst := cfg.Section("Desktop Entry").Key(ExecLocationKey).String()
			_, err = os.Stat(dst)
			if os.IsNotExist(err) {
				log.Println(dst, "does not exist, it is mentioned in", xdg.DataHome+"/applications/"+file.Name())
				continue
			}
			ai, err := NewAppImage(dst)
			if err != nil {
				continue
			}
			ui, err := ai.ReadUpdateInformation()
			if err == nil && ui != "" {
				//log.Println("updateinformation:", ui)
				// log.Println("updateinformation:", url.QueryEscape(ui))
				unescapedui, _ := url.QueryUnescape(ui)
				// log.Println("updateinformation:", unescapedui)
				if updateinformation == unescapedui {
					results = append(results, ai.Path)
				}
			}

			continue
		}
	}
	return results
}
