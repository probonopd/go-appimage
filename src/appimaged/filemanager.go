package main

// Adds context menus for AppImage to file managers
// https://github.com/AppImage/AppImageKit/issues/169
// Work in progress, not properly tested yet.
// TODO: Remove the context menus when appimaged exits

import (
	"os"
	"path/filepath"

	"github.com/probonopd/go-appimage/internal/helpers"

	"io/ioutil"

	"github.com/adrg/xdg"
)

arg0abs, err := filepath.Abs(os.Args[0])
if err != nil {
	log.Println(err)
}

// Context menus for the file manager in GNOME and KDE
// https://github.com/AppImage/AppImageKit/issues/169
func installFilemanagerContextMenus() {

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

	XFCEThunarAction := `<?xml encoding="UTF-8" version="1.0"?>
<actions>
<action>
    <icon>terminal</icon>
    <name>Update</name>
    <unique-id>1573903056061608-1</unique-id>
    <command>` + arg0abs + ` %f</command>
    <description>Update the AppImage</description>
    <patterns>*AppImage,*.appimage</patterns>
    <other-files/>
    <directories/>
</action>
</actions>
`
	// TODO: Nautilus (also) has $HOME/.local/share/nautilus/scripts

	// GNOME
	// https://github.com/Sadi58/nemo-actions
	// http://www.bernaerts-nicolas.fr/linux/76-gnome/344-nautilus-new-document-creation-menu
	// ~/.local/share/file-manager/actions/newfile-bash.desktop
	err := os.MkdirAll(xdg.DataHome+"/file-manager/actions/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}
	d1 := []byte(GNOMEFileManagerActionEntry)
	err = ioutil.WriteFile(xdg.DataHome+"/file-manager/actions/appimaged.desktop", d1, 0644)
	helpers.PrintError("filemanager", err)

	// KDE
	// $HOME/.local/share/kservices5/ServiceMenus/appimageupdate.desktop
	err = os.MkdirAll(xdg.DataHome+"/kservices5/ServiceMenus/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}
	d2 := []byte(KDEServiceMenuEntry)
	err = ioutil.WriteFile(xdg.DataHome+"/kservices5/ServiceMenus/appimaged.desktop", d2, 0644)
	helpers.PrintError("filemanager", err)

	// XFCE
	// Thunar allows users to add custom actions to the file and folder context menus
	// (by the use of the thunar-uca plugin, part of the Thunar distribution, in the plugins/ subdirectory).
	// FIXME: Do not overwrite pre-existing uca.xml file but insert the new action into it
	// This is more complicated than needed. Can't Thunar handle one file per action?s
	err = os.MkdirAll(xdg.ConfigHome+"/Thunar/", 0755)
	if err != nil {
		helpers.PrintError("filemanager", err)
	}
	d3 := []byte(XFCEThunarAction)
	err = ioutil.WriteFile(xdg.ConfigHome+"/Thunar/uca.xml", d3, 0644)
	helpers.PrintError("filemanager", err)

	// Cinnamon/Nemo
	// TODO: https://github.com/Sadi58/nemo-actions
}
