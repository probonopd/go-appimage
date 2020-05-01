package helpers

import (
	"errors"
	"fmt"
	"gopkg.in/ini.v1"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AppDir struct {
	Path            string
	DesktopFilePath string
	MainExecutable  string
}

func NewAppDir(desktopFilePath string) (AppDir, error) {
	var ad AppDir

	// Check if desktop file exists
	if Exists(desktopFilePath) == false {
		return ad, errors.New("Desktop file not found")
	}
	ad.DesktopFilePath = desktopFilePath

	// Determine root directory of the AppImage
	pathToBeChecked := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(ad.DesktopFilePath)))) + "/usr/bin"
	if Exists(pathToBeChecked) {
		ad.Path = filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(ad.DesktopFilePath))))
		fmt.Println("AppDir path:", ad.Path)
	} else {
		return ad, errors.New("AppDir could not be identified: " + pathToBeChecked + " does not exist")
	}

	// Copy the desktop file into the root of the AppDir
	ad.DesktopFilePath = desktopFilePath
	err := CopyFile(ad.DesktopFilePath, ad.Path+"/"+filepath.Base(ad.DesktopFilePath))
	if err != nil {
		return ad, err
	}

	// Find main top-level desktop file
	infos, err := ioutil.ReadDir(ad.Path)
	if err != nil {
		PrintError("ReadDir", err)
		return ad, err
	}
	var counter int
	for _, info := range infos {
		if err != nil {
			log.Printf("%v\n", err)
		}
		if strings.HasSuffix(info.Name(), ".desktop") == true {
			ad.DesktopFilePath = ad.Path + "/" + info.Name()
			counter = counter + 1
		}
	}

	// Return if we have too few or too many top-level desktop files now
	if counter < 1 {
		return ad, errors.New("No desktop file was found, please place one into " + ad.Path)
	}
	if counter > 1 {
		return ad, errors.New("More than one desktop file was found in " + ad.Path)
	}

	ini.PrettyFormat = false
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		ad.DesktopFilePath)
	if err != nil {
		return ad, err
	}

	sect, err := cfg.GetSection("Desktop Entry")
	if err != nil {
		return ad, err
	}

	if sect.HasKey("Exec") == false {
		err = errors.New("'Desktop Entry' section has no Exec= key")
		return ad, err
	}

	exec, err := sect.GetKey("Exec")
	if err != nil {
		return ad, err
	}

	// Desktop file verification
	err = CheckDesktopFile(ad.DesktopFilePath)
	if err != nil {
		return ad, err
	}

	// Do not allow paths in the Exec= key
	fmt.Println("Exec= key contains:", filepath.Base(strings.Split(exec.String(), " ")[0]))
	if strings.Split(exec.String(), " ")[0] != filepath.Base(strings.Split(exec.String(), " ")[0]) {
		err = errors.New("Exec= contains a path, please remove it")
		return ad, err
	}

	ad.MainExecutable = ad.Path + "/usr/bin/" + strings.Split(exec.String(), " ")[0] // TODO: Do not hardcode /usr/bin, instead search the AppDir for an executable file with that name?

	iconName, err := sect.GetKey("Icon")
	if err != nil {
		return ad, err
	}

	// Do not allow paths in the Icon= key
	fmt.Println("Icon= key contains:", filepath.Base(strings.Split(iconName.String(), " ")[0]))
	if strings.Split(iconName.String(), " ")[0] != filepath.Base(strings.Split(iconName.String(), " ")[0]) {
		err = errors.New("Icon= contains a path, please remove it")
		return ad, err
	}

	// Copy the main icon to the AppDir root directory if it is not there yet
	err = ad.CopyMainIconToRoot(iconName.String())
	if err != nil {
		return ad, err
	}

	return ad, nil
}

func (AppDir) GetElfInterpreter(appdir AppDir) (string, error) {
	cmd := exec.Command("patchelf", "--print-interpreter", appdir.MainExecutable)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// In this case, it might be that we have a script there that starts with a shebang
		// TODO: get binary from shebang (resolve to ELF absolute path)
		// and determine its ELF interpreter instead (or use the next best ELF binary in the AppDir)
		fmt.Println(cmd.String())
		PrintError("patchelf --print-interpreter "+appdir.MainExecutable+": "+string(out), err)
		return "", err
	}
	ldLinux := strings.TrimSpace(string(out))
	return ldLinux, nil
}

// CreateIconDirectories creates empty directories
// in <AppDir>/usr/share/icons/<size>/apps
func (appdir AppDir) CreateIconDirectories() (error) {
	// Only use the most common sizes in the hope that at least
	// those will work on all target systems
	iconSizes := []int{ 512, 256, 128, 48, 32, 24, 22, 16, 8 }
	var err error = nil
	for _, iconSize := range iconSizes {
		err = os.MkdirAll(appdir.Path+"/usr/share/icons/hicolor/"+string(iconSize)+"x"+string(iconSize)+"/apps", 0755)
	}
	return err
}

// CopyMainIconToRoot copies the most suitable icon for the
// Icon= entry in DesktopFilePath to the root of the AppDir
func (appdir AppDir) CopyMainIconToRoot(iconName string) (error) {
	var err error = nil
	iconPreferenceOrder := []int{ 128, 256, 512, 48, 32, 24, 22, 16, 8 }
	if Exists(appdir.Path + "/" + iconName+  ".png") {
		log.Println("Top-level icon already exists, leaving untouched")
	} else {
	for _, iconSize := range iconPreferenceOrder {
		candidate := appdir.Path+"/usr/share/icons/hicolor/"+string(iconSize)+"x"+string(iconSize)+"/apps/" + iconName + ".png"
		if Exists(candidate){
			CopyFile(candidate,appdir.Path + "/" + iconName+  ".png" )
		}
	}
	}
	return err
}
