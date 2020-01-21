package helpers

import (
	// "io"

	"os"
	"os/exec"
	"strings"
)

// Usage:
// func main() {
// 	cmd := []string{"ls", "-lh"}
// 	RunCmdTransparently(cmd)

// 	command := "ls -lh"
// 	RunCmdStringTransparently(command)
// }

// RunCmdTransparently runs the command given in a []string
// and runs it transparently (stdin and stderr appear
// immediately while the command is running).
// Blocks until command has completed. Returns error.
// https://stackoverflow.com/a/31004293
// https://stackoverflow.com/q/33452726
// Why is this not part of the standard library?
func RunCmdTransparently(command []string) error {
	cmd := exec.Command(command[0], command[1:]...)
	// Similar to the Unix tee(1) command.
	// Needed if we want to process the output further?
	// mwriter := io.MultiWriter(f, os.Stdout)
	cmd.Stdout = os.Stdout // or: mwriter
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run() // Blocks until complete
	return err

}

// RunCmdStringTransparently works like
// RunCmdTransparently but accepts a string rather than
// a []string as its input.
// Why is this not part of the standard library?
func RunCmdStringTransparently(command string) error {
	cmd := strings.Split(command, " ")
	return RunCmdTransparently(cmd)
}
