package goappimage

import (
	"bytes"

	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"io"

	"github.com/CalebQ42/squashfs"
	"github.com/probonopd/go-appimage/internal/helpers"
)

/*

TODO List:
* Provide a way to get the desktop file, or at least an ini.File representation of it.
* Provide a way to get thumbnail.
* Check if there IS an update
* Download said update

*/

// AppImage handles AppImage files.
// Currently it is using using a static build of mksquashfs/unsquashfs
// but eventually may be rewritten to do things natively in Go
//
// None of this is currently NEEDED to be exposed to the library user to edit.
type AppImage struct {
	path              string
	offset            int64
	updateInformation string
	niceName          string
	imageType         int
	reader            *squashfs.Reader
}

const execLocationKey = helpers.ExecLocationKey

// NewAppImage creates an AppImage object from the location defined by path.
// The AppImage object will also be created if path does not exist,
// because the AppImage that used to be there may need to be removed
// and for this the functions of an AppImage are needed.
// Non-existing and invalid AppImages will have type -1.
func NewAppImage(path string) AppImage {
	ai := AppImage{path: path, imageType: -1}
	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		return ai
	}
	ai.imageType = ai.determineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.imageType < 0 {
		return ai
	}
	ai.niceName = ai.calculateNiceName()
	if ai.imageType < 1 {
		return ai
	}
	if ai.imageType > 1 {
		ai.offset = helpers.CalculateElfSize(ai.path)
	}
	if ai.imageType == 2 {
		//Try to populate the ai.Reader to make it easier to use and get information.
		//The library is still very new, so we can fallback to command based functions if necessary.
		aiFil, err := os.Open(path)
		if err != nil {
			return ai
		}
		stat, err := aiFil.Stat()
		if err != nil {
			return ai
		}
		secReader := io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset)
		reader, err := squashfs.NewSquashfsReader(secReader)
		if err != nil {
			return ai
		}
		ai.reader = reader
		//TODO: possibly get some info (such as name) from the appimage's files, specifically it's desktop file.
	}
	return ai
}

//Name is the "nice" name of the AppImage.
func (ai AppImage) Name() string {
	return ai.niceName
}

func (ai AppImage) calculateNiceName() string {
	//TODO: have this as a fallback to reading the appimage's .desktop file
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

func runCommand(cmd *exec.Cmd, verbose bool) (io.Writer, error) {
	if verbose == true {
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
	if info.IsDir() {
		return -1
	}
	// Very small files cannot be AppImages, so return fast
	if info.Size() < 100*1024 {
		return -1
	}
	if helpers.CheckMagicAtOffset(f, "414902", 8) == true {
		return 2
	}
	if helpers.CheckMagicAtOffset(f, "414901", 8) == true {
		return 1
	}
	// ISO9660 files that are also ELF files
	if helpers.CheckMagicAtOffset(f, "7f454c", 0) == true && helpers.CheckMagicAtOffset(f, "4344303031", 32769) == true {
		return 1
	}
	return -1
}

// Validate checks the quality of an AppImage and sends desktop notification, returns error or nil
// TODO: Add more checks and reuse this in appimagetool
// func (ai AppImage) Validate(verbose bool) error {
// 	if verbose == true {
// 		log.Println("Validating AppImage", ai.path)
// 	}
// 	// Check validity of the updateinformation in this AppImage, if it contains some
// 	if ai.UpdateInformation != "" {
// 		log.Println("Validating updateinformation in", ai.path)
// 		err := helpers.ValidateUpdateInformation(ai.UpdateInformation)
// 		if err != nil {
// 			helpers.PrintError("appimage: updateinformation verification", err)
// 			return err
// 		}
// 	}
// 	return nil
// }

// ExtractFile extracts a file from from filepath (which may contain * wildcards)
// in an AppImage to the destinationdirpath.
// Returns err in case of errors, or nil.
// TODO: resolve symlinks
// TODO: Should this be a io.Reader()?
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string, verbose bool) error {
	var err error
	if ai.imageType == 1 {
		//TODO: possibly replace this with a library
		err = os.MkdirAll(destinationdirpath, os.ModePerm)
		cmd := exec.Command("bsdtar", "-C", destinationdirpath, "-xf", ai.path, filepath)
		_, err = runCommand(cmd, verbose)
		return err
	} else if ai.imageType == 2 {
		if ai.reader != nil {
			file := ai.reader.GetFileAtPath(filepath)
			if file == nil {
				goto commandFallback
			}
			errs := file.ExtractTo(destinationdirpath)
			if len(errs) > 0 {
				goto commandFallback
			}
			file.Close()
			return nil
		}
	commandFallback:
		cmd := exec.Command("unsquashfs", "-f", "-n", "-o", strconv.Itoa(int(ai.offset)), "-d", destinationdirpath, ai.path, filepath)
		_, err = runCommand(cmd, verbose)
		return err
	}
	// FIXME: What we may have extracted may well be (until here) broken symlinks... we need to do better than that
	return nil
}

// ReadUpdateInformation reads updateinformation from an AppImage
// Returns updateinformation string and error
//
// Not needed until a proper interface for upgrading is implemented.
func (ai AppImage) readUpdateInformation() (string, error) {
	aibytes, err := helpers.GetSectionData(ai.path, ".upd_info")
	if err != nil {
		return "", err
	}
	ui := strings.TrimSpace(string(bytes.Trim(aibytes, "\x00")))
	// Don't validate here, we don't want to get warnings all the time.
	// We have AppImage.Validate as its own function which we call less frequently than this.
	return ui, nil
}

// getFSTime reads FSTime from the AppImage. We are doing this only when it is needed,
// not when an NewAppImage is called
func (ai AppImage) getFSTime() time.Time {
	if ai.imageType == 1 {
		fil, err := os.Open(ai.path)
		if err != nil {
			return time.Unix(0, 0)
		}
		stat, _ := fil.Stat()
		return stat.ModTime()
	} else if ai.imageType == 2 {
		if ai.reader != nil {
			return ai.reader.ModTime()
		}
		result, err := exec.Command("unsquashfs", "-q", "-fstime", "-o", strconv.FormatInt(ai.offset, 10), ai.path).Output()
		resstr := strings.TrimSpace(string(bytes.TrimSpace(result)))
		if err != nil {
			helpers.PrintError("appimage: getFSTime: "+ai.path, err)
			return time.Unix(0, 0)
		}
		if n, err := strconv.Atoi(resstr); err == nil {
			return time.Unix(int64(n), 0)
		}
		log.Println("appimage: getFSTime:", resstr, "is not an integer.")
		return time.Unix(0, 0)
	}
	return time.Unix(0, 0)
}
