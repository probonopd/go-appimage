package fusemount

import (
	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

// Mounts the folder to the mount location as ReadOnly. Should be run in a goroutine. Uses fuse2 instead of fuse3.
func Fuse2Mount(folder string, mount string) (con *fuse.Conn, err error) {
	con, err = fuse.Mount(mount, fuse.ReadOnly())
	if err != nil {
		if con != nil {
			con.Close()
		}
		return
	}
	node := newFuse2FileNode(folder)
	err = fs.Serve(con, node)
	if err != nil {
		con.Close()
	}
	return
}
