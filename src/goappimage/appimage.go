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
	imageType, _ := determineImageType(path)
	return imageType != -1
}

// NewAppImage creates an AppImage object from the location defined by path.
// Returns an error if the given path is not an appimage, or is a temporary file.
// In all instances, will still return the AppImage.
func NewAppImage(path string) (ai *AppImage, err error) {
	imageType, typeReason := determineImageType(path)
	ai = &AppImage{Path: path, imageType: imageType}
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
		return ai, errors.New("given path is NOT an AppImage: " + typeReason)
	}
	if ai.imageType > 1 {
		ai.offset = helpers.CalculateElfSize(ai.Path)
	}
	if ai.imageType == 1 {
		ai.reader, err = newType1Reader(ai.Path)
	} else if ai.imageType == 2 {
		ai.reader, err = newType2Reader(ai)
	}
	if err != nil {
		return ai, errors.Join(errors.New("unable to create AppImage file reader"), err)
	}
	//try to load up the desktop file for some information.
	var desk string
	files := ai.reader.ListFiles(".")

	// Pick first .desktop without NoDisplay=true
	var (
		rdr       io.ReadCloser
		cfg       *ini.File
		firstDesk string // первый успешно распарсенный .desktop (для fallback)
		firstCfg  *ini.File
	)
	for _, f := range files {
		if !strings.HasSuffix(f, ".desktop") {
			continue
		}
		rdr, err = ai.reader.FileReader(f)
		if err != nil {
			continue
		}
		// Read entire file and escape semicolons before ini parsing
		raw, err := io.ReadAll(rdr)
		rdr.Close()
		if err != nil {
			continue
		}
		escaped := bytes.ReplaceAll(raw, []byte(";"), []byte("；"))

		cfg, err = ini.Load(escaped)
		if err != nil {
			continue
		}
		// remember first successfully parsed .desktop for fallback
		if firstDesk == "" {
			firstDesk = f
			firstCfg = cfg
		}
		if !cfg.Section("Desktop Entry").Key("NoDisplay").MustBool(false) {
			desk = f
			ai.Desktop = cfg // reuse already parsed config
			ai.Name = cfg.Section("Desktop Entry").Key("Name").Value()
			ai.Version = cfg.Section("Desktop Entry").Key("X-AppImage-Version").Value()
			break
		}
	}

	// Fallback: use first successfully parsed .desktop if all have NoDisplay=true
	if ai.Desktop == nil {
		if firstCfg != nil {
			// Already parsed — use directly, no need to re-parse
			desk = firstDesk
			ai.Desktop = firstCfg
			ai.Name = firstCfg.Section("Desktop Entry").Key("Name").Value()
			ai.Version = firstCfg.Section("Desktop Entry").Key("X-AppImage-Version").Value()
		} else {
			// No .desktop could be parsed at all — pick first one and try again
			for _, f := range files {
				if strings.HasSuffix(f, ".desktop") {
					desk = f
					break
				}
			}
			if desk != "" {
				var desktopFil io.ReadCloser
				desktopFil, err = ai.reader.FileReader(desk)
				if err != nil {
					return
				}

				// cleaning the desktop file so it can be parsed properly
				var desktop []byte
				buf := bufio.NewReader(desktopFil)
				for err == nil {
					var line string
					line, err = buf.ReadString('\n')
					line = strings.ReplaceAll(line, ";", "；")
					desktop = append(desktop, line...)
				}

				ai.Desktop, err = ini.Load(desktop)
				if err != nil {
					return
				}

				ai.Name = ai.Desktop.Section("Desktop Entry").Key("Name").Value()
				ai.Version = ai.Desktop.Section("Desktop Entry").Key("X-AppImage-Version").Value()
			}
		}
	}

	if desk == "" {
		return ai, errors.New("cannot find desktop file")
	}

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

// Check whether we have an AppImage at all, and if not, why.
// Return image type, or -1 + reason if it is not an AppImage.
func determineImageType(path string) (int, string) {
	// log.Println("appimage: ", ai.path)
	f, err := os.Open(path)
	// printError("appimage", err)
	if err != nil {
		// If we were not able to open the file, then we report that it is not an AppImage
		return -1, "could not open file: " + err.Error()
	}
	info, err := os.Stat(path)
	if err != nil {
		return -1, "could not stat file: " + err.Error()
	}
	// Directories cannot be AppImages, so return fast
	if info.IsDir() {
		return -1, "path is a directory"
	}
	// Very small files cannot be AppImages, so return fast
	if info.Size() < 100*1024 {
		return -1, "file is too small to be an AppImage"
	}
	if helpers.CheckMagicAtOffset(f, "414902", 8) {
		return 2, ""
	}
	if helpers.CheckMagicAtOffset(f, "414901", 8) {
		return 1, ""
	}
	// ISO9660 files that are also ELF files
	if helpers.CheckMagicAtOffset(f, "7f454c", 0) && helpers.CheckMagicAtOffset(f, "4344303031", 32769) {
		return 1, ""
	}
	return -1, "no AppImage magic bytes found (neither type 2 (0x414902) nor type 1 (0x414901) at offset 8, nor an ISO9660+ELF type-1 image)"
}

// SquashfsReader allows direct access to an AppImage's squashfs.
// Only works on type 2 AppImages
func (ai AppImage) SquashfsReader() (*squashfs.Reader, error) {
	if ai.imageType != 2 {
		return nil, errors.New("not a type 2 appimage")
	}
	aiFil, err := os.Open(ai.Path)
	if err != nil {
		return nil, err
	}
	squashRdr, err := squashfs.NewReaderAtOffset(aiFil, ai.offset)
	if err != nil {
		return nil, err
	}
	return squashRdr, nil
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

// ModTime is the time the AppImage was edited/created. If the AppImage is type 2,
// it will try to get that information from the squashfs, if not, it returns the file's ModTime.
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
