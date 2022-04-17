package goappimage

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
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
* Check if there IS an update
* Download said update

*/

// AppImage handles AppImage files.
type AppImage struct {
	reader archiveReader
	//Desktop is the AppImage's main .desktop file parsed as an ini.File.
	Desktop *ini.File
	Path    string
	// updateInformation string TODO: add update stuff
	Name      string
	Version   string
	offset    int64
	imageType int
}

// NewAppImage creates an AppImage object from the location defined by path.
// Returns an error if the given path is not an appimage, or is a temporary file.
// In all instances, will still return the AppImage.
func NewAppImage(path string) (*AppImage, error) {
	ai := AppImage{Path: path, imageType: -1}
	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		return &ai, errors.New("given path is a temporary file")
	}
	ai.imageType = ai.determineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.imageType < 0 {
		return &ai, errors.New("given path is NOT an AppImage")
	}
	if ai.imageType > 1 {
		ai.offset = helpers.CalculateElfSize(ai.Path)
	}
	err := ai.populateReader(true, false)
	if err != nil {
		return &ai, err
	}
	//try to load up the desktop file for some information.
	desktopFil, err := ai.reader.FileReader("*.desktop")
	if err != nil {
		return nil, err
	}

	//cleaning the desktop file so it can be parsed properly
	var desktop []byte
	buf := bufio.NewReader(desktopFil)
	for err == nil {
		var line string
		line, err = buf.ReadString('\n')
		if strings.Contains(line, ";") {
			line = strings.ReplaceAll(line, ";", "；") //replacing it with a fullwidth semicolon (unicode FF1B)
		}
		desktop = append(desktop, line...)
	}

	ai.Desktop, err = ini.Load(desktop)
	if err == nil {
		ai.Name = ai.Desktop.Section("Desktop Entry").Key("Name").Value()
		ai.Version = ai.Desktop.Section("Desktop Entry").Key("X-AppImage-Version").Value()
	}
	if ai.Name == "" {
		ai.Name = ai.calculateNiceName()
	}
	//If key "X-AppImage-Version" not set (likely), resort to just setting it to 1
	if ai.Version == "" {
		ai.Version = "1.0"
	}
	return &ai, nil
}

func (ai AppImage) calculateNiceName() string {
	niceName := filepath.Base(ai.Path)
	niceName = strings.Replace(niceName, ".AppImage", "", -1)
	niceName = strings.Replace(niceName, ".appimage", "", -1)
	niceName = strings.Replace(niceName, "-x86_64", "", -1)
	niceName = strings.Replace(niceName, "-i386", "", -1)
	niceName = strings.Replace(niceName, "-i686", "", -1)
	niceName = strings.Replace(niceName, "-aarch64", "", -1)
	niceName = strings.Replace(niceName, "-armhf", "", -1)
	niceName = strings.Replace(niceName, "-", " ", -1)
	niceName = strings.Replace(niceName, "_", " ", -1)
	return niceName
}

// Check whether we have an AppImage at all.
// Return image type, or -1 if it is not an AppImage
func (ai AppImage) determineImageType() int {
	// log.Println("appimage: ", ai.path)
	f, err := os.Open(ai.Path)
	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}
	info, err := os.Stat(ai.Path)
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

//SquashfsReader allows direct access to an AppImage's squashfs.
//Only works on type 2 AppImages
func (ai AppImage) SquashfsReader() (*squashfs.Reader, error) {
	if ai.imageType != 2 {
		return nil, errors.New("not a type 2 appimage")
	}
	aiFil, err := os.Open(ai.Path)
	if err != nil {
		return nil, err
	}
	stat, _ := aiFil.Stat()
	aiRdr := io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset)
	squashRdr, err := squashfs.NewSquashfsReader(aiRdr)
	if err != nil {
		return nil, err
	}
	return squashRdr, nil
}

//Type is the type of the AppImage. Should be either 1 or 2.
func (ai AppImage) Type() int {
	return ai.imageType
}

//ExtractFile extracts a file from from filepath (which may contain * wildcards) in an AppImage to the destinationdirpath.
//
//If resolveSymlinks is true, if the filepath specified is a symlink, the actual file is extracted in it's place.
//resolveSymlinks will have no effect on absolute symlinks (symlinks that start at root).
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string, resolveSymlinks bool) error {
	return ai.reader.ExtractTo(filepath, destinationdirpath, resolveSymlinks)
}

//ExtractFileReader tries to get an io.ReadCloser for the file at filepath.
//Returns an error if the path is pointing to a folder. If the path is pointing to a symlink,
//it will try to return the file being pointed to, but only if it's within the AppImage.
func (ai AppImage) ExtractFileReader(filepath string) (io.ReadCloser, error) {
	return ai.reader.FileReader(filepath)
}

//Thumbnail tries to get the AppImage's thumbnail and returns it as a io.ReadCloser.
func (ai AppImage) Thumbnail() (io.ReadCloser, error) {
	return ai.reader.FileReader(".DirIcon")
}

//Icon tries to get a io.ReadCloser for the icon dictated in the AppImage's desktop file.
//Returns the ReadCloser and the file's name (which could be useful for decoding).
func (ai AppImage) Icon() (io.ReadCloser, string, error) {
	if ai.Desktop == nil {
		return nil, "", errors.New("desktop file wasn't parsed")
	}
	icon := ai.Desktop.Section("Desktop Entry").Key("Icon").Value()
	if icon == "" {
		return nil, "", errors.New("desktop file doesn't specify an icon")
	}
	if strings.HasSuffix(icon, ".png") || strings.HasSuffix(icon, ".svg") {
		rdr, err := ai.reader.FileReader(icon)
		if err == nil {
			return rdr, icon, nil
		}
	}
	rootFils := ai.reader.ListFiles("/")
	for _, fil := range rootFils {
		if strings.HasPrefix(fil, icon) {
			if fil == icon+".png" {
				rdr, err := ai.reader.FileReader(fil)
				if err != nil {
					continue
				}
				return rdr, fil, nil
			} else if fil == icon+".svg" {
				rdr, err := ai.reader.FileReader(fil)
				if err != nil {
					continue
				}
				return rdr, fil, nil
			}
		}
	}
	return nil, "", errors.New("Cannot find the AppImage's icon: " + icon)
}

func (ai AppImage) Args() ([]string, error) {
	if ai.Desktop == nil {
		return nil, errors.New("desktop file wasn't parsed")
	}
	var exec = ai.Desktop.Section("Desktop Entry").Key("Exec").Value()
	if exec == "" {
		return nil, errors.New("exec key not present")
	}
	if strings.HasPrefix(exec, "\"") {
		if strings.Contains(exec[1:], "\"") {
			exec = exec[1 : strings.Index(exec[1:], "\"")+1]
		}
	}
	spl := strings.Split(exec, " ")
	if len(spl) <= 1 {
		return make([]string, 0), nil
	}
	return spl[1:], nil
}

func runCommand(cmd *exec.Cmd) (bytes.Buffer, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out, err
}

// TODO: implement update functionality
// ReadUpdateInformation reads updateinformation from an AppImage
// func (ai AppImage) readUpdateInformation() (string, error) {
// 	aibytes, err := helpers.GetSectionData(ai.path, ".upd_info")
// 	if err != nil {
// 		return "", err
// 	}
// 	ui := strings.TrimSpace(string(bytes.Trim(aibytes, "\x00")))
// 	return ui, nil
// }

//ModTime is the time the AppImage was edited/created. If the AppImage is type 2,
//it will try to get that information from the squashfs, if not, it returns the file's ModTime.
func (ai AppImage) ModTime() time.Time {
	if ai.imageType == 2 {
		if ai.reader != nil {
			return ai.reader.(*type2Reader).rdr.ModTime()
		}
		result, err := exec.Command("unsquashfs", "-q", "-fstime", "-o", strconv.FormatInt(ai.offset, 10), ai.Path).Output()
		resstr := strings.TrimSpace(string(bytes.TrimSpace(result)))
		if err != nil {
			goto fallback
		}
		if n, err := strconv.Atoi(resstr); err == nil {
			return time.Unix(int64(n), 0)
		}
	}
fallback:
	fil, err := os.Open(ai.Path)
	if err != nil {
		return time.Unix(0, 0)
	}
	stat, _ := fil.Stat()
	return stat.ModTime()
}
