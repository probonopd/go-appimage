package main

import (
	"fmt"
	"os/exec"
)

func main() {
	c := exec.Command("./mksquashfs", "../../../*.AppDir", "HI.sfs")
	err := c.Run()
	if err != nil {
		fmt.Println(err)
	}
}
