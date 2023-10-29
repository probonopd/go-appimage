// Blocked by
// https://github.com/diskfs/go-diskfs/issues/33

package main

// import (
// 	"log"
// 	"os"

// 	"github.com/diskfs/go-diskfs/filesystem/squashfs" // Have to use: GO111MODULE=on /usr/local/go/bin/go get github.com/diskfs/go-diskfs@squashfs
// )

// func readSquash() {

// 	f, err := os.Open("/home/me/Downloads/appimagetool-x86_64.AppImage")
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	defer f.Close()
// 	fs, err := squashfs.Read(f, 0, 188392, 131072)
// 	log.Println(fs.ReadDir("/"))
// }
