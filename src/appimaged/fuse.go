package main

import (
	"log"
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
	desktopPath := filepath.Join(xdg.DataHome, "applications/appimaged")
	makeSureExist := []string{
		cacheDir,
		desktopCache,
		thumbnailCache,
	}
	for _, v := range makeSureExist {
		err = os.MkdirAll(v, 0755)
		if err != nil && !os.IsExist(err) {
			return
		}
	}

	//Create desktopPath. If it already exists, make sure it's unmounted (from appimaged that wasn't properly closed)
	err = os.Mkdir(desktopPath, 0755)
	if os.IsExist(err) {
		if *verbosePtr {
			log.Println(desktopPath, "already exists, trying to umount in case appimaged was previously incorrectly closed")
			log.Println("running:", "umount", desktopPath)
		}
		exec.Command("umount", desktopPath).Run()
	} else if err != nil {
		return
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
