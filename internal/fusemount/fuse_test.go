package fusemount

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMount(t *testing.T) {
	exec.Command("umount", "./test").Run()
	os.Mkdir("test", 0755)
	wd, _ := os.Getwd()
	con, err := FuseMount(filepath.Join(wd, "../helpers"), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer con.Close()
	<-con.Ready
	t.Fatal("end")
}
