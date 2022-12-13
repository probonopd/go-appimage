package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/CalebQ42/fuse"
	"github.com/adrg/xdg"
	"github.com/probonopd/go-appimage/internal/fusemount"
	"github.com/probonopd/go-appimage/internal/helpers"
	fuse2 "github.com/seaweedfs/fuse"
)

func startFuse() (toDefer func(), err error) {
	makeSureExist := []string{
		cacheDir,
		desktopCache,
		thumbnailCache,
		filepath.Join(xdg.DataHome, "applications/appimaged"),
	}
	for _, v := range makeSureExist {
		err = os.MkdirAll(v, 0755)
		if err != nil && !os.IsExist(err) {
			return
		}
	}
	if _, err = exec.LookPath("fusermount3"); err == nil {
		var deskCon *fuse.Conn
		go func() {
			deskCon, err = fusemount.FuseMount(desktopCache, filepath.Join(xdg.DataHome, "applications/appimaged"))
			if err != nil {
				helpers.LogError("fuse mount", err)
			}
		}()
		return func() {
			if deskCon != nil {
				deskCon.Close()
			}
		}, nil
	}
	var deskCon *fuse2.Conn
	go func() {
		deskCon, err = fusemount.Fuse2Mount(desktopCache, filepath.Join(xdg.DataHome, "applications/appimaged"))
		if err != nil {
			helpers.LogError("fuse mount", err)
		}
	}()
	return func() {
		if deskCon != nil {
			deskCon.Close()
		}
	}, nil

}
