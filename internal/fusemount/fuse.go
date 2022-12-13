package fusemount

import (
	"github.com/CalebQ42/fuse"
	"github.com/CalebQ42/fuse/fs"
)

// Mounts the folder to the mount location as ReadOnly. Should be run in a goroutine
func FuseMount(folder string, mount string) (con *fuse.Conn, err error) {
	con, err = fuse.Mount(mount, fuse.ReadOnly())
	if err != nil {
		if con != nil {
			con.Close()
		}
		return
	}
	node := newFileNode(folder)
	err = fs.Serve(con, node)
	if err != nil {
		con.Close()
	}
	return
}
