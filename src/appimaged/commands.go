// Contains the commands appimaged can be invoked with
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/probonopd/appimage/internal/helpers"
)

func takeCareOfCommandlineCommands() {

	if len(os.Args) < 3 {
		fmt.Println("Not enough arguments supplied")
		os.Exit(1)
	}

	// As quickly as possible go there if we are invoked with the "notify" command
	if os.Args[1] == "notify" {
		JustNotify()
		os.Exit(0)
	}

	// As quickly as possible go there if we are invoked with the "wrap" command
	if os.Args[1] == "wrap" {
		appwrap()
		os.Exit(0)
	}

	// As quickly as possible go there if we are invoked with the "update" command
	if os.Args[1] == "update" {
		update()
		os.Exit(0)
	}

	// As quickly as possible run the most recent AppImage we can find if we are
	// invoked with the "run" command and updateinformation as arguments
	// appimaged run <updateinformation>: Waits for the process to exit
	// appimaged start <updateinformation>: Does not wait and exits immediately after having tried to launch
	if os.Args[1] == "run" || os.Args[1] == "start" {
		err := helpers.ValidateUpdateInformation(os.Args[2])
		var ui string
		if err == nil {
			ui = os.Args[2]
		} else {
			fmt.Println("Invalid updateinformation string supplied")
			os.Exit(1)
		}
		a := FindMostRecentAppImageWithMatchingUpdateInformation(ui)
		if a == "" {
			fmt.Println("No AppImage found for,")
		} else {
			comnd := []string{a}
			comnd = append(comnd, os.Args[3:]...)

			if os.Args[1] == "run" {
				helpers.RunCmdTransparently(comnd)
			} else {
				cmd := exec.Command(comnd[0], comnd[1:]...)
				err := cmd.Start()
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
	os.Exit(0) // Ensure that if we are in this function, then the process exits no matter what
}
