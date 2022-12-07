package fuse

import (
	"errors"
	iofs "io/fs"
	"os"

	"github.com/CalebQ42/fuse"
	"github.com/CalebQ42/fuse/fs"
)

func TransparentMount(folder *os.File, mount string) (con *fuse.Conn, err error) {
	var stat iofs.FileInfo
	if stat, err = folder.Stat(); err != nil || !stat.IsDir() {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("folder is not a folder")
	}
	con, err = fuse.Mount(mount, fuse.AllowNonEmptyMount(), fuse.FSName(""))
	if err != nil {
		if con != nil {
			con.Close()
		}
		return
	}
	err = fs.Serve(con, fileRoot{
		File: folder,
	})
	if err != nil {
		con.Close()
	}
	return
}
