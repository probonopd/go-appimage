package goappimage

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	Desktop    *ini.File
	Path       string
	Name       string
	UpdateInfo string
	Version    string
	offset     int64
	imageType  int
}

func IsAppImage(path string) bool {
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		return false
	}
	return determineImageType(path) != -1
}

// NewAppImage creates an AppImage object from the location defined by path.
// Returns an error if the given path is not an appimage, or is a temporary file.
// In all instances, will still return the AppImage.
func NewAppImage(path string) (ai *AppImage, err error) {
	ai = &AppImage{Path: path, imageType: determineImageType(path)}
	// If we got a temp file, exit immediately
	// E.g., ignore typical Internet browser temporary files used during download
	if strings.HasSuffix(path, ".temp") ||
		strings.HasSuffix(path, "~") ||
		strings.HasSuffix(path, ".part") ||
		strings.HasSuffix(path, ".partial") ||
		strings.HasSuffix(path, ".zs-old") ||
		strings.HasSuffix(path, ".crdownload") {
		return ai, errors.New("given path is a temporary file")
	}
	// Don't waste more time if the file is not actually an AppImage
	if ai.imageType < 0 {
		return ai, errors.New("given path is NOT an AppImage")
	}
	if ai.imageType > 1 {
		ai.offset = helpers.CalculateElfSize(ai.Path)
	}
	switch ai.imageType {
	case 1:
		ai.reader, err = newType1Reader(ai.Path)
	case 2:
		// Determine if AppImage is squashfs or dwarfs
		magic := make([]byte, 6)
		var aiFil *os.File
		aiFil, err = os.Open(ai.Path)
		if err != nil {
			return ai, err
		}
		_, err = aiFil.ReadAt(magic, ai.offset)
		if err != nil {
			return ai, err
		}
		if bytes.Equal(magic[:4], []byte("hsqs")) {
			ai.reader, err = newType2SquashfsReader(ai)
		} else if bytes.Equal(magic, []byte("DWARFS")) {
			ai.reader, err = newType2DwarfsReader(ai)
		} else {
			return ai, errors.New("type 2 appimage does not use squashfs or dwarfs which is currently unsupported")
		}
	}
	if err != nil {
		return ai, errors.Join(errors.New("unable to create AppImage file reader"), err)
	}
	//try to load up the desktop file for some information.
	var desk string
	files := ai.reader.ListFiles(".")
	for _, f := range files {
		if strings.HasSuffix(f, ".desktop") {
			desk = f
			break
		}
	}
	if desk == "" {
		return ai, errors.New("cannot find desktop file")
	}
	desktopFil, err := ai.reader.FileReader(desk)
	if err != nil {
		return
	}
	defer desktopFil.Close()

	//cleaning the desktop file so it can be parsed properly
	var desktop []byte
	buf := bufio.NewReader(desktopFil)
	for err == nil {
		var line string
		line, err = buf.ReadString('\n')
		line = strings.ReplaceAll(line, ";", "；") //replacing it with a fullwidth semicolon (unicode FF1B)
		desktop = append(desktop, line...)
	}

	ai.Desktop, err = ini.Load(desktop)
	if err != nil {
		return
	}

	ai.Name = ai.Desktop.Section("Desktop Entry").Key("Name").Value()
	ai.Version = ai.Desktop.Section("Desktop Entry").Key("X-AppImage-Version").Value()
	if ai.Name == "" {
		ai.Name = ai.calculateNiceName()
	}
	if ai.Version == "" {
		ai.Version = "1.0"
	}

	ai.UpdateInfo, _ = helpers.ReadUpdateInfo(ai.Path)
	return
}

func (ai AppImage) calculateNiceName() string {
	niceName := filepath.Base(ai.Path)
	niceName = strings.ReplaceAll(niceName, ".AppImage", "")
	niceName = strings.ReplaceAll(niceName, ".appimage", "")
	niceName = strings.ReplaceAll(niceName, ".app", "")
	niceName = strings.ReplaceAll(niceName, ".App", "")
	niceName = strings.ReplaceAll(niceName, "-x86_64", "")
	niceName = strings.ReplaceAll(niceName, "-i386", "")
	niceName = strings.ReplaceAll(niceName, "-i686", "")
	niceName = strings.ReplaceAll(niceName, "-aarch64", "")
	niceName = strings.ReplaceAll(niceName, "-armhf", "")
	niceName = strings.ReplaceAll(niceName, "-", " ")
	niceName = strings.ReplaceAll(niceName, "_", " ")
	return niceName
}

// Check whether we have an AppImage at all.
// Return image type, or -1 if it is not an AppImage
func determineImageType(path string) int {
	// log.Println("appimage: ", ai.path)
	f, err := os.Open(path)
	// printError("appimage", err)
	if err != nil {
		return -1 // If we were not able to open the file, then we report that it is not an AppImage
	}
	info, err := os.Stat(path)
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

// SquashfsReader allows direct access to an AppImage's squashfs.
// Only works on type 2 AppImages that use squashfs
func (ai AppImage) SquashfsReader() (*squashfs.Reader, error) {
	if sfsRdr, ok := ai.reader.(*type2SquashfsReader); ok {
		return sfsRdr.rdr, nil
	}
	return nil, errors.New("not a type 2 appimage with squashfs")
}

// Type is the type of the AppImage. Should be either 1 or 2.
func (ai AppImage) Type() int {
	return ai.imageType
}

func (ai AppImage) ListFiles(folder string) []string {
	return ai.reader.ListFiles(folder)
}

// ExtractFile extracts a file from from filepath (which may contain * wildcards) in an AppImage to the destinationdirpath.
//
// If resolveSymlinks is true, if the filepath specified is a symlink, the actual file is extracted in it's place.
// resolveSymlinks will have no effect on absolute symlinks (symlinks that start at root).
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string, resolveSymlinks bool) error {
	return ai.reader.ExtractTo(filepath, destinationdirpath, resolveSymlinks)
}

// ExtractFileReader tries to get an io.ReadCloser for the file at filepath.
// Returns an error if the path is pointing to a folder. If the path is pointing to a symlink,
// it will try to return the file being pointed to, but only if it's within the AppImage.
func (ai AppImage) ExtractFileReader(filepath string) (io.ReadCloser, error) {
	return ai.reader.FileReader(filepath)
}

// Thumbnail tries to get the AppImage's thumbnail and returns it as a io.ReadCloser.
func (ai AppImage) Thumbnail() (io.ReadCloser, error) {
	return ai.reader.FileReader(".DirIcon")
}

// Icon tries to get a io.ReadCloser for the icon dictated in the AppImage's desktop file.
// Returns the ReadCloser and the file's name (which could be useful for decoding).
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
	pngs, svgs := []string{}, []string{}
	rootFils := ai.reader.ListFiles("/")
	for _, fil := range rootFils {
		if strings.HasSuffix(fil, ".png") {
			pngs = append(pngs, fil)
		} else if strings.HasSuffix(fil, ".svg") {
			svgs = append(svgs, fil)
		}
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
	if len(pngs) > 0 {
		rdr, err := ai.reader.FileReader(pngs[0])
		if err == nil {
			return rdr, pngs[0], nil
		}
	} else if len(svgs) > 0 {
		rdr, err := ai.reader.FileReader(svgs[0])
		if err == nil {
			return rdr, svgs[0], nil
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

// ModTime is the time the AppImage was edited/created. If the AppImage is type 2,
// it will try to get that information from the squashfs, if not, it returns the file's ModTime.
func (ai AppImage) ModTime() time.Time {
	if sfsRdr, ok := ai.reader.(*type2SquashfsReader); ok {
		return sfsRdr.rdr.ModTime()
	}
	fil, err := os.Open(ai.Path)
	if err != nil {
		return time.Unix(0, 0)
	}
	stat, _ := fil.Stat()
	return stat.ModTime()
}
