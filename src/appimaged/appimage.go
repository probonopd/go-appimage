package main

import (
	"C"

	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/url"

	"gopkg.in/ini.v1"

	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/go-language-server/uri"
	helpers "github.com/probonopd/appimage/internal/helpers"
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
	updateinformation string
	niceName          string
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
	ai.niceName = ai.calculateNiceName()
	if ai.imagetype < 1 {
		return ai
	}
	ai.offset = helpers.CalculateElfSize(ai.path)
	ui, err := ai.ReadUpdateInformation()
	if err == nil && ui != "" {
		ai.updateinformation = ui
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
func (ai AppImage) discoverContents() {
	// Let's get the listing of files inside the AppImage. We can work on this later on
	// to resolve symlinks, and to determine which files to extract in addition to the desktop file and icon
	cmd := exec.Command("")
	if ai.imagetype == 1 {
		cmd = exec.Command("bsdtar", "-t", ai.path)
	} else if ai.imagetype == 2 {
		cmd = exec.Command("unsquashfs", "-f", "-n", "-ll", "-o", strconv.FormatInt(ai.offset, 10), "-d ''", ai.path)
	}
	if *verbosePtr == true {
		log.Printf("cmd: %q\n", cmd.String())
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	helpers.LogError("appimage: list files:", err)
	ai.rawcontents = out.String()
}

func (ai AppImage) calculateMD5filenamepart() string {
	hasher := md5.New()
	hasher.Write([]byte(ai.uri))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (ai AppImage) calculateNiceName() string {
	niceName := filepath.Base(ai.path)
	niceName = strings.Replace(niceName, ".AppImage", "", -1)
	niceName = strings.Replace(niceName, ".appimage", "", -1)
	niceName = strings.Replace(niceName, "-x86_64", "", -1)
	niceName = strings.Replace(niceName, "-i386", "", -1)
	niceName = strings.Replace(niceName, "-i686", "", -1)
	niceName = strings.Replace(niceName, "-", " ", -1)
	niceName = strings.Replace(niceName, "_", " ", -1)
	return niceName
}

func runCommand(cmd *exec.Cmd) (io.Writer, error) {
	if *verbosePtr == true {
		log.Printf("runCommand: %q\n", cmd)
	}
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

	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}

	info, err := os.Stat(ai.path)
	if err != nil {
		return -1
	}

	// Directories cannot be AppImages, so return fast
	if err == nil && info.IsDir() {
		return -1
	}

	// Very small files cannot be AppImages, so return fast
	if err == nil && info.Size() < 100*1024 {
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
	helpers.LogError("checkMagicAtOffset: "+f.Name(), err)
	b := make([]byte, len(magic)/2) // Read bytes
	n, err := f.Read(b)
	helpers.LogError("checkMagicAtOffset: "+f.Name(), err)
	hexmagic := hex.EncodeToString(b[:n])
	if hexmagic == magic {
		// if *verbosePtr == true {
		// 	log.Printf("checkMagicAtOffset: %v: Magic 0x%x at offset %v\n", f.Name(), string(b[:n]), offset)
		// }
		return true
	}
	return false
}

func (ai AppImage) setExecBit() {

	err := os.Chmod(ai.path, 0755)
	if err == nil {
		if *verbosePtr == true {
			log.Println("appimage: Set executable bit on", ai.path)
		}
	}
	// printError("appimage", err) // Do not print error since AppImages on read-only media are common
}

// CheckQualityAndNotify checks the quality of an AppImage and notifies about any issues found
// Returns error or nil
func (ai AppImage) Validate() error {
	if *verbosePtr == true {
		log.Println("Validating AppImage", ai.path)
	}
	// Check validity of the updateinformation in this AppImage, if it contains some
	if ai.updateinformation != "" {
		if *verbosePtr == true {
			log.Println("Validating updateinformation in", ai.path)
		}
		err := helpers.ValidateUpdateInformation(ai.updateinformation)
		helpers.PrintError("appimage: updateinformation verification", err)
		if err != nil {
			sendDesktopNotification("Invalid AppImage", ai.niceName+"\ncontains invalid update information:\n"+ai.updateinformation+"\n"+err.Error()+"\nPlease ask the author to fix it.", 30000)
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
					// but subscribe
					go SubscribeMQTT(MQTTclient, ai.updateinformation)
					return
				}
			}
		}
	}

	// Let's be evil and integrate only good AppImages...
	err := ai.Validate()
	if err != nil {
		log.Println("AppImage did not pass validation:", ai.path)
		return
	}

	writeDesktopFile(ai) // Do not run with "go" as it would interfere with extractDirIconAsThumbnail

	// Subscribe to MQTT messages for this application
	if ai.updateinformation != "" {
		go SubscribeMQTT(MQTTclient, ai.updateinformation)
	}

	// SimpleNotify(ai.path, "Integrated", 3000)

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

// Do not call this directly. Instead, call IntegrateOrUnintegrate
func (ai AppImage) _removeIntegration() {
	log.Println("appimage: Remove integration", ai.path)
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
		sendDesktopNotification("Removed", ai.path, 3000)

	}
}

// IntegrateOrUnintegrate integrates or unintegrates
// (registers or unregisters) an AppImage from the system,
// depending on whether the file exists on disk
func (ai AppImage) IntegrateOrUnintegrate() {

	if _, err := os.Stat(ai.path); os.IsNotExist(err) {
		ai._removeIntegration()
	} else {
		ai._integrate()
	}
}

func ioReader(file string) io.ReaderAt {
	r, err := os.Open(file)
	defer r.Close()
	helpers.LogError("appimage: elf:", err)
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

// ExtractFile extracts a file from from filepath (which may contain * wildcards)
// in an AppImage to the destinationdirpath.
// Returns err in case of errors, or nil.
// TODO: resolve symlinks
// TODO: Should this be a io.Reader()?
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string) error {
	var err error
	if ai.imagetype == 1 {
		err = os.MkdirAll(destinationdirpath, os.ModePerm)
		cmd := exec.Command("bsdtar", "-C", destinationdirpath, "-xf", ai.path, filepath)
		_, err = runCommand(cmd)
		return err
	} else if ai.imagetype == 2 {
		cmd := exec.Command("unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.offset, 10), "-d", destinationdirpath, ai.path, filepath)
		_, err = runCommand(cmd)
		return err
	}
	// FIXME: What we may have extracted may well be (until here) broken symlinks... we need to do better than that
	return nil
}

// ReadUpdateInformation reads updateinformation from an AppImage
// Returns updateinformation string and error
func (ai AppImage) ReadUpdateInformation() (string, error) {
	aibytes, err := helpers.GetSectionData(ai.path, ".upd_info")
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
		helpers.RunCmdTransparently(cmd)

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
			cfg, e := ini.Load(xdg.DataHome + "/applications/" + file.Name())
			helpers.LogError("desktop", e)
			dst := cfg.Section("Desktop Entry").Key(ExecLocationKey).String()
			_, err = os.Stat(dst)
			if os.IsNotExist(err) {
				log.Println(dst, "does not exist, it is mentioned in", xdg.DataHome+"/applications/"+file.Name())
				continue
			}
			ai := NewAppImage(dst)
			ui, err := ai.ReadUpdateInformation()
			if err == nil && ui != "" {
				//log.Println("updateinformation:", ui)
				// log.Println("updateinformation:", url.QueryEscape(ui))
				unescapedui, _ := url.QueryUnescape(ui)
				// log.Println("updateinformation:", unescapedui)
				if updateinformation == unescapedui {
					results = append(results, ai.path)
				}
			}

			continue
		}
	}
	return results
}

// getFSTime reads FSTime from the AppImage. We are doing this only when it is needed,
// not when an NewAppImage is called
func (ai AppImage) getFSTime() time.Time {
	if ai.imagetype == 2 {
		result, err := exec.Command("unsquashfs", "-q", "-fstime", "-o", strconv.FormatInt(ai.offset, 10), ai.path).Output()
		resstr := strings.TrimSpace(string(bytes.TrimSpace(result)))
		if err != nil {
			helpers.PrintError("appimage: getFSTime: "+ai.path, err)
			return time.Unix(0, 0)
		}
		if n, err := strconv.Atoi(resstr); err == nil {
			return time.Unix(int64(n), 0)
		} else {
			log.Println("appimage: getFSTime:", resstr, "is not an integer.")
			return time.Unix(0, 0)
		}
	} else {
		log.Println("TODO: Implement getFSTime for type-1 AppImages")
		return time.Unix(0, 0)
	}
}
