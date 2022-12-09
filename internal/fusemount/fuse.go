package fusemount

import (
	"github.com/CalebQ42/fuse"
	"github.com/CalebQ42/fuse/fs"
)

func FuseMount(folder string, mount string) (con *fuse.Conn, err error) {
	con, err = fuse.Mount(mount, fuse.AllowNonEmptyMount(), fuse.ReadOnly())
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
