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
	Name         string
	Version      string
	Permissions *Permissions
	offset       int64
	imageType    int
}

// Simple, Android-like permissions an AppImage can request
// Note that these are ONLY for external sandbox implementations to use, no
// sandboxing is done by default
type Permissions struct {
	Level     int    // How much access to system files
	Files   []string // Grant permission to access files
	Devices []string // Access device files (eg: dri)
	Sockets []string // Use sockets (eg: x11, network)
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

	ai.Permissions, _ = loadPerms(ai.Desktop)

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

// Load permissions from INI
func loadPerms(f *ini.File) (*Permissions, error) {
	p := &Permissions{}
	var err error

	// Get permissions from keys
	level       := f.Section("X-AppImage-Required-Permissions").Key("Level").Value()
	filePerms   := f.Section("X-AppImage-Required-Permissions").Key("Files").Value()
	devicePerms := f.Section("X-AppImage-Required-Permissions").Key("Devices").Value()
	socketPerms := f.Section("X-AppImage-Required-Permissions").Key("Sockets").Value()

	if level != "" {
		l, err := strconv.Atoi(level)

		if err != nil || l < 0 || l > 3 {
			p.Level = -1
			return p, errors.New("invalid permissions level (must be 0-3)")
		} else {
			p.Level = l
		}
	} else {
		p.Level = -1
		return p, errors.New("profile does not have required flag `Level` under section [X-AppImage-Required-Permissions]")
	}

	p.Files = helpers.SplitKey(filePerms)
	p.Devices = helpers.SplitKey(devicePerms)
	p.Sockets = helpers.SplitKey(socketPerms)

	// Assume readonly if unspecified
	for i := range(p.Files) {
		ex := p.Files[i][len(p.Files[i])-3:]

		if len(strings.Split(p.Files[i], ":")) < 2 ||
		ex != ":ro" && ex != ":rw" {
			p.Files[i] = p.Files[i]+":ro"
		}
	}

	// Convert devices to shorthand if not already
	for i, val := range(p.Devices) {
		if len(val) > 5 && val[0:5] == "/dev/" {
			p.Devices[i] = strings.Replace(val, "/dev/", "", 1)
		}
	}

	return p, err
}

//Type is the type of the AppImage. Should be either 1 or 2.
func (ai AppImage) Type() int {
	return ai.imageType
}

//ExtractFile extracts a file from from filepath (which may contain * wildcards) in an AppImage to the destinationdirpath.
//
//If resolveSymlinks is true, if the filepath specified is a symlink, the actual file is extracted in its place.
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
