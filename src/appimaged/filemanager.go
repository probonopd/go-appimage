package main

// Adds context menus for AppImage to file managers
// https://github.com/AppImage/AppImageKit/issues/169
// Work in progress, not properly tested yet.
// TODO: Remove the context menus when appimaged exits

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/probonopd/go-appimage/internal/helpers"

	"github.com/adrg/xdg"
)

// Context menus for the file manager in GNOME and KDE
// https://github.com/AppImage/AppImageKit/issues/169
func installFilemanagerContextMenus() {
	arg0abs, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Println(err)
	}
	GNOMEFileManagerActionEntry := `[Desktop Entry]
Type=Action
Name=Update
Icon=terminal
TargetLocation=true
TargetToolbar=true
TargetContext=true
MimeType=application/vnd.appimage;
Capabilities=Writable
Profiles=directory;

[X-Action-Profile directory]
Exec=` + arg0abs + ` update %f
`

	KDEServiceMenuEntry := `[Desktop Entry]
Type=Service
X-KDE-ServiceTypes=KonqPopupMenu/Plugin
MimeType=application/vnd.appimage;
Actions=AppImageExecutable;AppImageUpdate;

[Desktop Action AppImageUpdate]
TryExec=AppImageUpdate
Exec=konsole -e AppImageUpdate %f
Icon=utilities-terminal
Name=Update
Comment=Update the AppImage

[Desktop Action AppImageExecutable]
Exec=` + arg0abs + ` update %f
Icon=utilities-terminal
Name=Make executable
`

	XFCEThunarActionUniqueID := `1573903056061608-1`

	XFCEThunarAction := `<action>
    <icon>terminal</icon>
    <name>Update</name>
    <unique-id>` + XFCEThunarActionUniqueID + `</unique-id>
    <command>` + arg0abs + ` %f</command>
    <description>Update the AppImage</description>
    <patterns>*.AppImage;*.appimage</patterns>
    <other-files/>
    <directories/>
</action>`

	XFCEThunarUCABody := `<?xml version="1.0" encoding="UTF-8"?>
<actions>
%s
</actions>	
`
	// TODO: Nautilus (also) has $HOME/.local/share/nautilus/scripts

	// GNOME
	// https://github.com/Sadi58/nemo-actions
	// http://www.bernaerts-nicolas.fr/linux/76-gnome/344-nautilus-new-document-creation-menu
	// ~/.local/share/file-manager/actions/newfile-bash.desktop
	err = os.MkdirAll(xdg.DataHome+"/file-manager/actions/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}
	d1 := []byte(GNOMEFileManagerActionEntry)
	err = os.WriteFile(xdg.DataHome+"/file-manager/actions/appimaged.desktop", d1, 0644)
	helpers.PrintError("filemanager", err)

	// KDE
	// $HOME/.local/share/kservices5/ServiceMenus/appimageupdate.desktop
	err = os.MkdirAll(xdg.DataHome+"/kservices5/ServiceMenus/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}
	d2 := []byte(KDEServiceMenuEntry)
	err = os.WriteFile(xdg.DataHome+"/kservices5/ServiceMenus/appimaged.desktop", d2, 0644)
	helpers.PrintError("filemanager", err)

	// XFCE
	// Thunar allows users to add custom actions to the file and folder context menus
	// (by the use of the thunar-uca plugin, part of the Thunar distribution, in the plugins/ subdirectory).
	err = os.MkdirAll(xdg.ConfigHome+"/Thunar/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}

	XFCEThunarUCABuffer := ""
	d3 := []byte(nil)

	if _, err = os.Stat(xdg.ConfigHome + "/Thunar/uca.xml"); os.IsNotExist(err) {
		// uca.xml doesn't exist so we write our default one
		d3 = []byte(fmt.Sprintf(XFCEThunarAction, XFCEThunarUCABody))
	} else {
		// uca.xml exists so we open it to:
		// A. Check if our action exists
		// B. If not, add it
		var ucaFile *os.File
		ucaFile, err = os.Open(xdg.ConfigHome + "/Thunar/uca.xml")
		if err != nil {
			helpers.PrintError("filemanager", err)
		}
		defer ucaFile.Close()

		ucaFileReader := bufio.NewScanner(ucaFile)

		for ucaFileReader.Scan() { // We read the file line by line checking for our unique-id
			curLine := ucaFileReader.Text()
			if strings.Contains(curLine, XFCEThunarActionUniqueID) { // If we find our action, there's nothing to be done so we return
				return
			}
		}

		// If we don't find our action, we reset the file to the beginning and reinstantiate the scanner
		ucaFile.Seek(0, 0)
		ucaFileReader = bufio.NewScanner(ucaFile)

		// Then we add our action
		curLine := ucaFileReader.Text() // Read the XML header into curLine
		for curLine != "<actions>" {    // Read everything up to the actions section into the buffer
			ucaFileReader.Scan()
			curLine = ucaFileReader.Text()
			XFCEThunarUCABuffer += curLine + "\n"
		}
		XFCEThunarUCABuffer += XFCEThunarAction + "\n" // Add the update action
		for ucaFileReader.Scan() {                     // Read the rest of the file into our buffer
			XFCEThunarUCABuffer += ucaFileReader.Text() + "\n"
		}

		d3 = []byte(XFCEThunarUCABuffer)
	}
	err = os.WriteFile(xdg.ConfigHome+"/Thunar/uca.xml", d3, 0644)
	helpers.PrintError("filemanager", err)

	// Cinnamon/Nemo
	// TODO: https://github.com/Sadi58/nemo-actions
}
