package helpers

import (
	"bytes"
	"debug/elf"
	"fmt"
	"github.com/adrg/xdg"
	version "github.com/hashicorp/go-version"
	"gopkg.in/ini.v1"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// Here returns the location of the executable based on os.Args[0]
func Here() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
		return ""
	}
	return (dir)
}

// AddHereToPath adds the location of the executable to the $PATH
func AddHereToPath() {
	// The directory we run from is added to the $PATH so that we find helper
	// binaries there, too
	os.Setenv("PATH", Here()+":"+os.Getenv("PATH"))
	log.Println("main: PATH:", os.Getenv("PATH"))
}

// IsCommandAvailable returns true if a file is on the $PATH
func IsCommandAvailable(name string) bool {
	cmd := exec.Command("/bin/sh", "-c", "command -v "+name)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
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
	cfg, e := ini.Load(desktopfilepath)
	LogError("desktop", e)
	dst := cfg.Section("Desktop Entry").Key("Exec").String()

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
	in, err := os.Open(src)
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

// WriteFileIntoOtherFileAtOffset writes the content of inputfile into outputfile at offset, without truncating
// Returns error in case of errors, otherwise returns nil
func WriteFileIntoOtherFileAtOffset(inputfilepath string, outputfilepath string, offset uint64) (error) {
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
	fo.Seek(int64(offset), 0)

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

// WriteStringIntoOtherFileAtOffset writes the content of inputstring into outputfile at offset, without truncating
// Returns error in case of errors, otherwise returns nil
func WriteStringIntoOtherFileAtOffset(inputstring string, outputfilepath string, offset uint64) (error) {
	fo, err := os.OpenFile(outputfilepath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fo.Seek(int64(offset), 0)
	defer fo.Close()
	buf := bytes.NewBufferString(inputstring)
		if _, err := buf.WriteTo(fo); err != nil {
			return err
		}
	return nil
}

// GetSectionData returns the contents of an ELF section and error
func GetSectionData(filepath string, name string) ([]byte, error) {
	r, err := os.Open(filepath)
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

// GetSectionOffsetAndLength returns the offset and length of an ELF section and error
func GetSectionOffsetAndLength(filepath string, name string) (uint64, uint64, error) {
	r, err := os.Open(filepath)
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