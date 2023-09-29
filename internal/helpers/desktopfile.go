package helpers

import (
	"errors"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

func CheckDesktopFile(desktopfile string) error {
	// Check for presence of required keys and abort otherwise
	d, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		desktopfile)
	PrintError("ini.load", err)
	neededKeys := []string{"Categories", "Name", "Exec", "Type", "Icon"}
	for _, k := range neededKeys {
		if !d.Section("Desktop Entry").HasKey(k) {
			return errors.New(".desktop file is missing a '" + k + "'= key\n")
		}
	}

	val, _ := d.Section("Desktop Entry").GetKey("Icon")
	iconname := val.String()
	if strings.Contains(iconname, "/") {
		return errors.New("desktop file contains Icon= entry with a path")
	}

	if strings.HasSuffix(filepath.Base(iconname), ".png") ||
		strings.HasSuffix(filepath.Base(iconname), ".svg") ||
		strings.HasSuffix(filepath.Base(iconname), ".svgz") ||
		strings.HasSuffix(filepath.Base(iconname), ".xpm") {
		return errors.New("desktop file contains Icon= entry with a suffix, please remove the suffix")
	}

	return nil
}
