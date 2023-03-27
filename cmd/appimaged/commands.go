// Contains the commands appimaged can be invoked with
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/probonopd/go-appimage/internal/helpers"
)

func HandleCommands() {
	switch flag.Arg(0) {
	case "wrap":
		appwrap()
	case "update":
		update()
	case "run":
		runFromUpdateInfo(flag.Args()[1:], true)
	case "start":
		runFromUpdateInfo(flag.Args()[1:], false)
	default:
		fmt.Println("Invalid command")
		os.Exit(1)
	}
}

func runFromUpdateInfo(args []string, wait bool) {
	if len(args) == 0 {
		return
	}
	err := helpers.ValidateUpdateInformation(args[0])
	if err != nil {
		fmt.Println("Invalid updateinformation string supplied")
		os.Exit(1)
	}
	a := FindMostRecentAppImageWithMatchingUpdateInformation(args[0])
	if a == "" {
		fmt.Println("No AppImage found for given update information")
		os.Exit(1)
	}
	cmd := exec.Command(a, args[1:]...)
	if wait {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		err = cmd.Start()
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
