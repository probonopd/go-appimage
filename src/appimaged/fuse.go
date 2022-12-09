package main

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/probonopd/go-appimage/internal/fusemount"
)

func startFuse() (toDefer func(), err error) {
	makeSureExist := []string{
		cacheDir,
		desktopCache,
		filepath.Join(xdg.DataHome, "applications/appimaged"),
	}
	for _, v := range makeSureExist {
		err = os.MkdirAll(v, 0755)
		if err != nil && !os.IsExist(err) {
			return
		}
	}
	deskCon, err := fusemount.FuseMount(desktopCache, filepath.Join(xdg.DataHome, "applications/appimaged"))
	if err != nil {
		return
	}
	return func() {
		deskCon.Close()
	}, nil
}
