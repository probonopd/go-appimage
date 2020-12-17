package goappimage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CalebQ42/squashfs"
	"github.com/probonopd/go-appimage/internal/helpers"
	"gopkg.in/ini.v1"
)

/*

TODO List:
* Provide a way to get the desktop file, or at least an ini.File representation of it.
* Provide a way to get thumbnail.
* Check if there IS an update
* Download said update

*/

// AppImage handles AppImage files.
type AppImage struct {
	reader            *squashfs.Reader
	Desktop           *ini.File
	path              string
	updateInformation string
	Name              string
	offset            int64
	imageType         int //The AppImages main .desktop file as an ini.File. Only available on type 2 AppImages right now.
}

const execLocationKey = helpers.ExecLocationKey

// NewAppImage creates an AppImage object from the location defined by path.
// The AppImage object will also be created if path does not exist,
// because the AppImage that used to be there may need to be removed
// and for this the functions of an AppImage are needed.
// Non-existing and invalid AppImages will have type -1.
func NewAppImage(path string) (*AppImage, error) {
	ai := AppImage{path: path, imageType: -1}
	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		return nil, errors.New("Given path is a temporary file")
	}
	ai.imageType = ai.determineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.imageType < 0 {
		return nil, errors.New("Given path is NOT an AppImage")
	}
	if ai.imageType > 1 {
		ai.offset = helpers.CalculateElfSize(ai.path)
	}
	if ai.imageType == 2 {
		//Try to populate the ai.Reader to make it easier to use and get information.
		//The library is still very new, so we can always fallback to command based functions if necessary.
		aiFil, _ := os.Open(path)
		stat, err := aiFil.Stat()
		if err != nil {
			return &ai, err
		}
		reader, err := squashfs.NewSquashfsReader(io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset))
		if err != nil {
			return &ai, nil
		}
		ai.reader = reader
		//try to load up the desktop file for some information.
		desktopFil := reader.GetFileAtPath("*.desktop")
		if desktopFil != nil {
			defer desktopFil.Close()
			ai.Desktop, err = ini.Load(desktopFil)
			if err == nil {
				ai.Name = ai.Desktop.Section("Desktop Entry").Key("Name").Value()
			}
		}
	}
	if ai.Name == "" {
		ai.Name = ai.calculateNiceName()
	}
	return &ai, nil
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

//ExtractFile extracts a file from from filepath (which may contain * wildcards) in an AppImage to the destinationdirpath.
//
//If resolveSymlinks is true, if the filepath specified is a symlink, the actual file is extracted in it's place.
//On type 2 AppImages, this behavior is recursive if extracting a folder.
//resolveSymlinks will have no effect on absolute symlinks (symlinks that start at root).
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string, resolveSymlinks bool) error {
	var err error
	if ai.imageType == 1 {
		err = os.MkdirAll(destinationdirpath, os.ModePerm)
		if err != nil {
			return err
		}
		name := path.Base(filepath)
		filepath = strings.TrimPrefix(filepath, "/")
		destinationdirpath = strings.TrimSuffix(destinationdirpath, "/")
		tmpDir := destinationdirpath + "/" + ".temp"
		err = os.Mkdir(tmpDir, os.ModePerm)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		if resolveSymlinks {
			filepath, err = ai.getSymlinkLocation(filepath)
			if err != nil {
				return err //The only way to get an error is if the bsdtar command spits out an error for filepath, so actual extraction will fail.
			}
		}
		cmd := exec.Command("bsdtar", "-C", tmpDir, "-xf", ai.path, filepath)
		_, err = runCommand(cmd)
		if err != nil {
			return err
		}
		err = os.Rename(tmpDir+"/"+filepath, destinationdirpath+"/"+name)
		if err != nil {
			return err
		}
		return err
	} else if ai.imageType == 2 {
		if ai.reader != nil {
			file := ai.reader.GetFileAtPath(filepath)
			if file == nil {
				goto commandFallback
			}
			var errs []error
			if resolveSymlinks {
				errs = file.ExtractSymlink(destinationdirpath)
			} else {
				errs = file.ExtractTo(destinationdirpath)
			}
			if len(errs) > 0 {
				goto commandFallback
			}
			file.Close()
			return nil
		}
	commandFallback:
		cmd := exec.Command("unsquashfs", "-f", "-n", "-o", strconv.Itoa(int(ai.offset)), "-d", destinationdirpath, ai.path, filepath)
		_, err = runCommand(cmd)
		return err
	}
	// FIXME: What we may have extracted may well be (until here) broken symlinks... we need to do better than that
	return nil
}

//ExtractFileReader tries to get an io.ReadCloser for the file at filepath.
//Returns an error if the path is pointing to a folder. If the path is pointing to a symlink,
//it will try to return the file being pointed to, but only if it's within the AppImage.
//
//This will try to use a native Go library, but if that fails it will fallback to using
//unsquashfs or bsdtar by extracting the file to the temp directory (defined by os.TempDir)
//that gets deleted when close is called on the returned ReadCloser.
func (ai AppImage) ExtractFileReader(filepath string) (io.ReadCloser, error) {
	if ai.reader != nil {
		fil := ai.reader.GetFileAtPath(filepath)
		if fil == nil {
			goto commandFallback
		}
		if fil.IsSymlink() {
			fil = fil.GetSymlinkFile()
		}
		return fil, nil
	}
	if ai.imageType == 1 {
	}
commandFallback:
	// This will allows us to fallback to commands if necessary for either type.
	// Will probably extract the file to a temp file using os.TempFile and delete it when Close() is called.
	if ai.imageType == 2 {
	}
	return nil, errors.New("Uh Oh")
}

//Icon tries to get the AppImage's icon and returns it as a io.ReadCloser.
func (ai AppImage) Icon() (io.ReadCloser, error) {
	if ai.imageType == 1 {
		//TODO
	} else if ai.imageType == 2 {
		if ai.reader != nil {
			iconFil := ai.reader.GetFileAtPath(".DirIcon")
			if iconFil == nil {
				goto commandFallback
			}
			if iconFil.IsSymlink() {
				iconFil = iconFil.GetSymlinkFile()
				if iconFil == nil {
					//If we've gotten this far, the reader is probably working properly and shouldn't fallback to commands.
					return nil, errors.New("Icon is a symlink to a file outside the AppImage") //TODO: give the path to where it's pointing
				}
			}
			return iconFil, nil
		}
	commandFallback:
		//TODO
	}
	return nil, errors.New("Icon couldn't be found")
}

func runCommand(cmd *exec.Cmd) (bytes.Buffer, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out, err
}

// ReadUpdateInformation reads updateinformation from an AppImage
func (ai AppImage) readUpdateInformation() (string, error) {
	aibytes, err := helpers.GetSectionData(ai.path, ".upd_info")
	if err != nil {
		return "", err
	}
	ui := strings.TrimSpace(string(bytes.Trim(aibytes, "\x00")))
	return ui, nil
}

//ModTime is the time the AppImage was edited/created. If the AppImage is type 2,
//it will try to get that information from the squashfs, if not, it returns the file's ModTime.
func (ai AppImage) ModTime() time.Time {
	if ai.reader != nil {
		return ai.reader.ModTime()
	}
	if ai.imageType == 2 {
		result, err := exec.Command("unsquashfs", "-q", "-fstime", "-o", strconv.FormatInt(ai.offset, 10), ai.path).Output()
		resstr := strings.TrimSpace(string(bytes.TrimSpace(result)))
		if err != nil {
			goto fallback
		}
		if n, err := strconv.Atoi(resstr); err == nil {
			return time.Unix(int64(n), 0)
		}
	}
fallback:
	fil, err := os.Open(ai.path)
	if err != nil {
		return time.Unix(0, 0)
	}
	stat, _ := fil.Stat()
	return stat.ModTime()
}
