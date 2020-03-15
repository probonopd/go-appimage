package helpers

import (
	"bytes"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/hashicorp/go-version"
	"gopkg.in/ini.v1"
)

// This key in the desktop files written by appimaged describes where the AppImage is in the filesystem.
// We need this because we rewrite Exec= to include things like wrap and Firejail
const ExecLocationKey = "X-ExecLocation"

// This key in the desktop files written by appimaged contains the updateinformation string
// that we write there so that we can get all updateinformation strings easily when
// we get a MQTT message telling us that an update is available. Then we need to find
// quickly all AppImages that have a matching updateinformation string, and figure out
// which of the existing ones is the newest
const UpdateInformationKey = "X-AppImage-UpdateInformation"

// PrintError prints error, prefixed by a string that explains the context
func PrintError(context string, e error) {
	if e != nil {
		os.Stderr.WriteString("ERROR " + context + ": " + e.Error() + "\n")
	}
}

// LogError logs error, prefixed by a string that explains the context
func LogError(context string, e error) {
	if e != nil {
		l := log.New(os.Stderr, "", 1)
		l.Println("ERROR " + context + ": " + e.Error())
	}
}

// Here returns the absolute path to the parent directory
// of the executable based on /proc/self/exe
// This will only work on Linux. For AppImages, it will resolve to the
// inside of an AppImage
func Here() string {
	fi, _ := os.Readlink("/proc/self/exe")
	return filepath.Dir(fi)
}

// HereArgs0 returns the absolute path to the parent directory
// of the executable based on os.Args[0]
// For AppImages, it will resolve to the outside of an AppImage
func HereArgs0() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
		return ""
	}
	return dir
}

// Args0 returns the absolute path to the executable based on os.Args[0]
// For AppImages, it will resolve to the outside of an AppImage
func Args0() string {
	dir, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Println(err)
		return ""
	}
	return dir
}

// AddDirsToPath adds the directories in []string to the $PATH
func AddDirsToPath(dirs []string) {
	for _, dir := range dirs {
		err := os.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		if err != nil {
			PrintError("helpers: AddHereToPath", err)
		}
	}
	log.Println("main: PATH:", os.Getenv("PATH"))
}

// AddHereToPath adds the location of the executable to the $PATH
func AddHereToPath() {
	// The directory we run from is added to the $PATH so that we find helper
	// binaries there, too
	err := os.Setenv("PATH", Here()+":"+os.Getenv("PATH"))
	if err != nil {
		PrintError("helpers: AddHereToPath", err)
	}
	// log.Println("main: PATH:", os.Getenv("PATH"))
}

// FilesWithSuffixInDirectoryRecursive returns the files in a given directory with the given filename extension, and err
func FilesWithSuffixInDirectoryRecursive(directory string, extension string) []string {
	var foundfiles []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(info.Name(), extension) {
			foundfiles = append(foundfiles, path)
		}
		return nil
	})
	if err != nil {
		return foundfiles
	}

	return foundfiles
}

// FilesWithSuffixInDirectory returns the files in a given directory with the given filename extension, and err
func FilesWithSuffixInDirectory(directory string, extension string) []string {
	var foundfiles []string
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return foundfiles
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), extension) {
			foundfiles = append(foundfiles, directory+"/"+file.Name())
		}
	}
	return foundfiles
}

// FilesWithPrefixInDirectory returns the files in a given directory with the given filename extension, and err
func FilesWithPrefixInDirectory(directory string, prefix string) []string {
	var foundfiles []string
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return foundfiles
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) {
			foundfiles = append(foundfiles, directory+"/"+file.Name())
		}
	}
	return foundfiles
}

// CheckIfFileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
// Returns true if it does, false otherwise.
func CheckIfFileExists(filepath string) bool {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) || info.IsDir() {
		return false
	}
	return true
}

// CheckIfExecFileExists checks whether a desktop file
// that points to an-existing Exec= entries.
// Returns true if it does, false otherwise.
func CheckIfExecFileExists(desktopfilepath string) bool {
	_, err := os.Stat(desktopfilepath)
	if os.IsNotExist(err) {
		return false
	}
	cfg, e := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		desktopfilepath)
	LogError("desktop", e)
	dst := cfg.Section("Desktop Entry").Key(ExecLocationKey).String()

	_, err = os.Stat(dst)
	if os.IsNotExist(err) {
		log.Println(dst, "does not exist, it is mentioned in", desktopfilepath)
		return false
	}
	return true
}

// DeleteDesktopFilesWithNonExistingTargets deletes desktop files
// in xdg.DataHome + "/applications/"
// that point to non-existing Exec= entries
func DeleteDesktopFilesWithNonExistingTargets() {
	files, e := ioutil.ReadDir(xdg.DataHome + "/applications/")
	LogError("desktop", e)
	if e != nil {
		return
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") && strings.HasPrefix(file.Name(), "appimagekit_") {
			exists := CheckIfExecFileExists(xdg.DataHome + "/applications/" + file.Name())
			if exists == false {
				log.Println("Deleting", xdg.DataHome+"/applications/"+file.Name())
				e = os.Remove(xdg.DataHome + "/applications/" + file.Name())
				LogError("desktop", e)
			}
		}
	}
}

// GetValuesForAllDesktopFiles gets the values for a given key from all desktop files
// in xdg.DataHome + "/applications/"
func GetValuesForAllDesktopFiles(key string) []string {
	var results []string
	files, e := ioutil.ReadDir(xdg.DataHome + "/applications/")
	LogError("GetValuesForAllDesktopFiles", e)
	if e != nil {
		return results
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") {
			exists := CheckIfExecFileExists(xdg.DataHome + "/applications/" + file.Name())
			if exists == true {
				cfg, e := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
					xdg.DataHome+"/applications/"+file.Name())
				LogError("GetValuesForAllDesktopFiles", e)
				dst := cfg.Section("Desktop Entry").Key(key).String()
				if dst != "" {
					results = append(results, dst)
				}
			}
		}
	}
	return results
}

// ValidateDesktopFile validates a desktop file using the desktop-file-validate tool on the $PATH
// Returns error if validation fails and prints any errors to stderr
func ValidateDesktopFile(desktopfile string) error {
	cmd := exec.Command("desktop-file-validate", desktopfile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("desktop-file-validate", err)
		fmt.Printf("%s", string(out))
		os.Stderr.WriteString("ERROR: Desktop file contains errors. Please fix them. Please see https://standards.freedesktop.org/desktop-entry-spec/1.0\n")
		return err
	}
	return nil
}

// ValidateAppStreamMetainfoFile validates an AppStream metainfo file using the appstreamcli tool on the $PATH
// Returns error if validation fails and prints any errors to stderr
func ValidateAppStreamMetainfoFile(appdirpath string) error {
	// Validate_desktop_file
	cmd := exec.Command("appstreamcli", "validate-tree", appdirpath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		PrintError("appstreamcli", err)
		fmt.Printf("%s", string(out))
		os.Stderr.WriteString("ERROR: AppStream metainfo file file contains errors. Please fix them. Please see ")
		os.Stderr.WriteString("https://www.freedesktop.org/software/appstream/docs/chap-Quickstart.html#sect-Quickstart-DesktopApps\n")
		return err
	}
	return nil
}

// CopyFile copies the src file to dst.
// Any existing file will be overwritten and will not
// copy file attributes.
// Unclear why such basic functionality is not in the standard library.
func CopyFile(src string, dst string) error {

	// We may have a symlink, so first resolve it
	srcResolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return err
	}

	in, err := os.Open(srcResolved)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// CheckIfSquashfsVersionSufficient checks whether mksquashfs/unsquashfs
// is recent enough to use -offset, prints an error message otherwise
// Returns true if sufficient, false otherwise
func CheckIfSquashfsVersionSufficient(toolname string) bool {
	cmd := exec.Command(toolname, "-version")
	out, err := cmd.CombinedOutput()
	// Interestingly unsquashfs 4.4 does not return with 0, unlike mksquashfs 4.3
	if strings.Contains(string(out), "version") == false {
		PrintError(toolname, err)
		fmt.Printf("%s", string(out))
		return false
	}
	parts := strings.Split(string(out), " ")
	ver := parts[2]
	if strings.Contains(ver, "-") {
		parts = strings.Split(ver, "-")
		ver = parts[0]
	}
	v1, err := version.NewVersion(ver)
	v2, err := version.NewVersion("4.4")
	if v1.LessThan(v2) {
		fmt.Println(toolname, "on the $PATH is version", v1, "but we need at least version 4.4, exiting")
		return false
	}
	return true
}

// WriteFileIntoOtherFileAtOffset writes the content of inputfile into outputfile at Offset, without truncating
// Returns error in case of errors, otherwise returns nil
func WriteFileIntoOtherFileAtOffset(inputfilepath string, outputfilepath string, offset uint64) error {
	// open input file
	f, err := os.Open(inputfilepath)
	if err != nil {
		return err
	}
	defer f.Close()

	// open output file
	fo, err := os.OpenFile(outputfilepath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fo.Close()
	_, err = fo.Seek(int64(offset), 0)
	if err != nil {
		return err
	}
	// make a buffer to keep chunks that are read
	buf := make([]byte, 1024)
	for {
		// read a chunk
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		// write a chunk
		if _, err := fo.Write(buf[:n]); err != nil {
			return err
		}
	}
	return nil
}

// WriteStringIntoOtherFileAtOffset writes the content of inputstring into outputfile at Offset, without truncating
// Returns error in case of errors, otherwise returns nil
func WriteStringIntoOtherFileAtOffset(inputstring string, outputfilepath string, offset uint64) error {
	fo, err := os.OpenFile(outputfilepath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = fo.Seek(int64(offset), 0)
	if err != nil {
		return err
	}
	defer fo.Close()
	buf := bytes.NewBufferString(inputstring)
	if _, err := buf.WriteTo(fo); err != nil {
		return err
	}
	return nil
}

// GetSectionData returns the contents of an ELF section and error
func GetSectionData(filepath string, name string) ([]byte, error) {
	// fmt.Println("GetSectionData for '" + name + "'")
	r, err := os.Open(filepath)
	if err == nil {
		defer r.Close()
	}
	f, err := elf.NewFile(r)
	if err != nil {
		return nil, err
	}
	section := f.Section(name)
	if section == nil {
		return nil, nil
	}
	data, err := section.Data()
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetSectionOffsetAndLength returns the Offset and Length of an ELF section and error
func GetSectionOffsetAndLength(filepath string, name string) (uint64, uint64, error) {
	r, err := os.Open(filepath)
	if err == nil {
		defer r.Close()
	}
	f, err := elf.NewFile(r)
	if err != nil {
		return 0, 0, err
	}
	section := f.Section(name)
	if section == nil {
		return 0, 0, nil
	}
	return section.Offset, section.Size, nil
}

// GetElfArchitecture returns the architecture of a file, and err
func GetElfArchitecture(filepath string) (string, error) {
	r, err := os.Open(filepath)
	if err == nil {
		defer r.Close()
	}
	f, err := elf.NewFile(r)
	if err != nil {
		return "", err
	}
	arch := f.Machine.String()
	// Why does everyone name architectures differently?
	switch arch {
	case "EM_X86_64":
		arch = "x86_64"
	case "EM_386":
		arch = "i686"
	case "EM_ARM":
		arch = "armhf"
	case "EM_AARCH64":
		arch = "aarch64"
	}
	return arch, nil
}

func AppendIfMissing(slice []string, s string) []string {
	for _, ele := range slice {
		if ele == s {
			return slice
		}
	}
	// fmt.Println("*** Appending", s)
	slice = append(slice, s)
	// fmt.Println( "*** Slice now contains", slice)
	return slice
}

// ReplaceTextInFile replaces search string with replce string in a file.
// Returns error or nil
func ReplaceTextInFile(path string, search string, replace string) error {
	input, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	output := bytes.Replace(input, []byte(search), []byte(replace), -1)
	if err = ioutil.WriteFile(path, output, 0666); err != nil {
		return err
	}
	return nil
}

// FindMostRecentFile returns the most recent file
// from a slice of files, (currently) based on its mtime
// based on https://stackoverflow.com/a/45579190
// TODO: mtime may be fast, but is it "good enough" for our purposes?
func FindMostRecentFile(files []string) string {
	var modTime time.Time
	var names []string
	for _, f := range files {
		fi, _ := os.Stat(f)
		if fi.Mode().IsRegular() {
			if !fi.ModTime().Before(modTime) {
				if fi.ModTime().After(modTime) {
					modTime = fi.ModTime()
					names = names[:0]
				}
				names = append(names, f)
			}
		}
	}
	if len(names) > 0 {
		fmt.Println(modTime, names[0]) // Most recent
		return names[0]                // Most recent
	}
	return ""
}

// Check for needed files on $PATH. Returns err
func CheckForNeededTools(tools []string) error {
	for _, t := range tools {
		_, err := exec.LookPath(t)
		if err != nil {
			log.Println("Required helper tool", t, "missing")
			return os.ErrNotExist
		}
	}
	return nil
}

// IsCommandAvailable returns true if a file is on the $PATH
func IsCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	if err == nil {
		return true
	}
	return false
}

// SliceContains returns true if the []string contains string,
// false otherwise.
// Why is this not in the standard library as a method of every []string?
func SliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Returns true if file or directory exists
// Why is this not in the standard library?
func Exists(name string) bool {
	_, err := os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Returns true if path is a directory
// Why is this not in the standard library?
func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

/* DOES NOT WORK PROPERLY
// Returns true if path is a file
// Why is this not in the standard library?
func IsFile(path string) (bool) {
	fdir, err := os.Open(path)
	if err != nil {
		return false
	}
	defer fdir.Close()

	finfo, err := fdir.Stat()
	if err != nil {
		return false
	}
	switch mode := finfo.Mode(); {

	case mode.IsRegular():
		return true
	}
	return false
}

*/

// Return true if magic string (hex) is found at offset
// TODO: Instead of magic string, could probably use something like []byte{'\r', '\n'} or []byte("AI")
func CheckMagicAtOffset(f *os.File, magic string, offset int64) bool {
	_, err := f.Seek(offset, 0) // Go to offset
	LogError("CheckMagicAtOffset: "+f.Name(), err)
	b := make([]byte, len(magic)/2) // Read bytes
	n, err := f.Read(b)
	LogError("CheckMagicAtOffset: "+f.Name(), err)
	hexmagic := hex.EncodeToString(b[:n])
	if hexmagic == magic {
		// if *verbosePtr == true {
		// 	log.Printf("CheckMagicAtOffset: %v: Magic 0x%x at offset %v\n", f.Name(), string(b[:n]), offset)
		// }
		return true
	}
	return false
}
