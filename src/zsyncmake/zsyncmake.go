package main

import (
	"flag"
	"fmt"
	"github.com/agriardyan/go-zsyncmake/zsync"
	"os"
	"path/filepath"
)

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Please provide the path to the file")
		os.Exit(1)
	}

	opts := zsync.Options{0, "", filepath.Base(flag.Args()[0])}
	zsync.ZsyncMake(flag.Args()[0], opts)
}
