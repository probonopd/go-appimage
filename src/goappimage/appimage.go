package goappimage

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
	"strconv"
	"strings"
	"time"

	"io"

	"github.com/CalebQ42/squashfs"
	"github.com/adrg/xdg"
	"github.com/probonopd/go-appimage/internal/helpers"
	"go.lsp.dev/uri"
)

// AppImage handles AppImage files.
// Currently it is using using a static build of mksquashfs/unsquashfs
// but eventually may be rewritten to do things natively in Go
type AppImage struct {
	Path              string
	ImageType         int
	URI               string
	Md5               string
	Offset            int64
	UpdateInformation string
	NiceName          string
	reader            *squashfs.Reader
}

const execLocationKey = helpers.ExecLocationKey

// NewAppImage creates an AppImage object from the location defined by path.
// The AppImage object will also be created if path does not exist,
// because the AppImage that used to be there may need to be removed
// and for this the functions of an AppImage are needed.
// Non-existing and invalid AppImages will have type -1.
func NewAppImage(path string) AppImage {

	ai := AppImage{Path: path, ImageType: -1}

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
	ai.URI = strings.TrimSpace(string(uri.File(filepath.Clean(ai.Path))))
	ai.Md5 = ai.calculateMD5filenamepart() // Need this also for non-existing AppImages for removal
	ai.ImageType = ai.determineImageType()
	// Don't waste more time if the file is not actually an AppImage
	if ai.ImageType < 0 {
		return ai
	}
	ai.NiceName = ai.calculateNiceName()
	if ai.ImageType < 1 {
		return ai
	}
	if ai.ImageType > 1 {
		ai.Offset = helpers.CalculateElfSize(ai.Path)
	}
	if ai.ImageType == 2 {
		//Run this in an inline func so we can handle errors more elegently. If there's a problem with the library, the unsquashfs tool will probably still work.
		ai.reader = func() *squashfs.Reader {
			aiFil, err := os.Open(path)
			if err != nil {
				return nil
			}
			stat, err := aiFil.Stat()
			if err != nil {
				return nil
			}
			secReader := io.NewSectionReader(aiFil, ai.Offset, stat.Size()-ai.Offset)
			reader, err := squashfs.NewSquashfsReader(secReader)
			if err != nil {
				return nil
			}
			return reader
		}()
	}
	ui, err := ai.ReadUpdateInformation()
	if err == nil && ui != "" {
		ai.UpdateInformation = ui
	}
	// ai.discoverContents() // Only do when really needed since this is slow
	// log.Println("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXX rawcontents:", ai.rawcontents)
	// Besides, for whatever reason it is not working properly yet

	return ai
}

func (ai AppImage) calculateMD5filenamepart() string {
	hasher := md5.New()
	hasher.Write([]byte(ai.URI))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (ai AppImage) calculateNiceName() string {
	niceName := filepath.Base(ai.Path)
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

// func (ai AppImage) setExecBit(verbose bool) {

// 	err := os.Chmod(ai.Path, 0755)
// 	if err == nil {
// 		if verbose == true {
// 			log.Println("appimage: Set executable bit on", ai.Path)
// 		}
// 	}
// 	// printError("appimage", err) // Do not print error since AppImages on read-only media are common
// }

// Validate checks the quality of an AppImage and sends desktop notification, returns error or nil
// TODO: Add more checks and reuse this in appimagetool
// func (ai AppImage) Validate(verbose bool) error {
// 	if verbose == true {
// 		log.Println("Validating AppImage", ai.Path)
// 	}
// 	// Check validity of the updateinformation in this AppImage, if it contains some
// 	if ai.UpdateInformation != "" {
// 		log.Println("Validating updateinformation in", ai.Path)
// 		err := helpers.ValidateUpdateInformation(ai.UpdateInformation)
// 		if err != nil {
// 			helpers.PrintError("appimage: updateinformation verification", err)
// 			return err
// 		}
// 	}
// 	return nil
// }

// func ioReader(file string) io.ReaderAt {
// 	r, err := os.Open(file)
// 	defer r.Close()
// 	helpers.LogError("appimage: elf:", err)
// 	return r
// }

// ExtractFile extracts a file from from filepath (which may contain * wildcards)
// in an AppImage to the destinationdirpath.
// Returns err in case of errors, or nil.
// TODO: resolve symlinks
// TODO: Should this be a io.Reader()?
func (ai AppImage) ExtractFile(filepath string, destinationdirpath string, verbose bool) error {
	var err error
	if ai.ImageType == 1 {
		err = os.MkdirAll(destinationdirpath, os.ModePerm)
		cmd := exec.Command("bsdtar", "-C", destinationdirpath, "-xf", ai.Path, filepath)
		_, err = runCommand(cmd, verbose)
		return err
	} else if ai.ImageType == 2 {
		if ai.reader != nil {
			file := ai.reader.GetFileAtPath(filepath)
			if file != nil { //so we can fall back to command based extraction.
				errs := file.ExtractTo(destinationdirpath)
				if len(errs) != 0 {
					//just return the first error
					return errs[0]
				}
				return nil
			}
		}
		cmd := exec.Command("unsquashfs", "-f", "-n", "-o", strconv.FormatInt(ai.Offset, 10), "-d", destinationdirpath, ai.Path, filepath)
		_, err = runCommand(cmd, verbose)
		return err
	}
	// FIXME: What we may have extracted may well be (until here) broken symlinks... we need to do better than that
	return nil
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
func LaunchMostRecentAppImage(updateinformation string, args []string, quiet bool) {
	if updateinformation == "" {
		return
	}
	if quiet == false {
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
			dst := cfg.Section("Desktop Entry").Key(execLocationKey).String()
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
					results = append(results, ai.Path)
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
	if ai.ImageType == 2 {
		result, err := exec.Command("unsquashfs", "-q", "-fstime", "-o", strconv.FormatInt(ai.Offset, 10), ai.Path).Output()
		resstr := strings.TrimSpace(string(bytes.TrimSpace(result)))
		if err != nil {
			helpers.PrintError("appimage: getFSTime: "+ai.Path, err)
			return time.Unix(0, 0)
		}
		if n, err := strconv.Atoi(resstr); err == nil {
			return time.Unix(int64(n), 0)
		}
		log.Println("appimage: getFSTime:", resstr, "is not an integer.")
		return time.Unix(0, 0)
	}
	log.Println("TODO: Implement getFSTime for type-1 AppImages")
	return time.Unix(0, 0)
}
