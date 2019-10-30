package main

import (
        "github.com/agriardyan/go-zsyncmake/zsync"
        "flag"
        "path/filepath"
        "fmt"
        "os"
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
