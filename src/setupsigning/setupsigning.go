// Encrypting and uploading a private key
// for signing AppImages with Travis CI
// without needing the travis command line tool
// Based on https://gist.github.com/kzap/5819745 and https://docs.travis-ci.com/user/encrypting-files/
//
// TODO: Use https://github.com/src-d/go-git or https://github.com/google/go-github to
// * Get info about the repo this is supposed to run in
// * Commit the encrypted file
// * even git push?

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/probonopd/appimage/internal/helpers"
	"github.com/shuheiktgw/go-travis"
)

var overwritePtr = flag.Bool("o", false, "Overwrite pre-existing files")

func main() {

	flag.Parse()
	// Check for needed files on $PATH
	tools := []string{"sh", "ssh-keygen", "openssl"}
	err := helpers.CheckForNeededTools(tools)
	if err != nil {
		os.Exit(1)
	}

	// Find out the URL to the GitHub repository
	cmd := "git remote -v | grep push | head -n 1 | cut -d ' ' -f 1"
	co := exec.Command("sh", "-c", cmd)
	res, err := co.Output()
	if err != nil {
		fmt.Println("Could not read git information")
		os.Exit(1)
	}
	if len(res) < 3 {
		fmt.Println("Could not read git information. Are you in a directory under git control?")
		os.Exit(1)
	}
	if strings.Contains(string(res), "origin") == false {
		fmt.Println("Could not interpret git information. Are you in a git repository cloned from GitHub?")
		os.Exit(1)
	}
	resstr := strings.TrimSpace(string(strings.Replace(string(res), "origin", "", -1)))
	// fmt.Println(resstr)
	parts := strings.Split(resstr, "/")
	repo_slug := strings.Join(parts[len(parts)-2:], "/")
	fmt.Println(repo_slug)

	// Check if private key already exists, delete it if -o was specified, exit otherwise
	if _, err := os.Stat("privkey"); err == nil {
		if *overwritePtr == false {
			fmt.Println("Private key already exists, exiting")
			os.Exit(1)
		} else {
			err := os.Remove("privkey")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	// Make a key with an empty passphrase
	// = generate public/private rsa key pair
	// FIXME: Does RunCmdTransparently swallow empty arguments?
	// cmds := []string{"ssh-keygen", "-t", "rsa", "-f", "./privkey", "-P", "\"\""}
	// helpers.RunCmdTransparently(cmds)
	// results in:
	// Saving key "./privkey" failed: passphrase is too short (minimum five characters)
	// Hence doing it differently:
	co = exec.Command("ssh-keygen", "-f", "id_rsa", "-t", "rsa", "-f", "./privkey", "-P", "")
	co.Run()

	// Check if we succeeded until here
	if _, err := os.Stat("privkey"); err != nil {
		fmt.Println("Could not create private key, exiting")
		os.Exit(1)
	}

	// Check if password/secret already exists, delete it if -o was specified, exit otherwise
	if _, err := os.Stat("secret"); err == nil {
		if *overwritePtr == false {
			fmt.Println("Secret already exists, exiting")
			os.Exit(1)
		} else {
			err := os.Remove("secret")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	// Generate the password/secret you will store encrypted in the .travis.yml and use to encrypt your private key
	cmd = "cat /dev/urandom | head -c 10000 | openssl sha1 | cut -d ' ' -f 2 > ./secret"
	co = exec.Command("sh", "-c", cmd)
	co.Run()

	// Check if we succeeded until here
	if _, err := os.Stat("secret"); err != nil {
		fmt.Println("Could not create secret, exiting")
		os.Exit(1)
	}

	// Read the super_secret_password file into a variable
	buf, err := ioutil.ReadFile("secret")
	if err != nil {
		fmt.Println("Could not read secret, exiting")
		os.Exit(1)
	}
	super_secret_password := strings.TrimSpace(string(buf))

	// Check if encrypted private key already exists, delete it if -o was specified, exit otherwise
	if _, err := os.Stat("privkey.enc"); err == nil {
		if *overwritePtr == false {
			fmt.Println("Encrypted private key already exists, exiting")
			os.Exit(1)
		} else {
			err := os.Remove("privkey.enc")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	// Encrypt your private key using your secret password
	cmd = "openssl aes-256-cbc -pass file:./secret -in ./privkey -out ./privkey.enc -a"
	helpers.RunCmdStringTransparently(cmd)

	// Check if we succeeded until here
	if _, err := os.Stat("privkey.enc"); err != nil {
		fmt.Println("Could not encrypt private key, exiting")
		os.Exit(1)
	}

	// Delete unneeded public key
	os.Remove("privkey.pub")

	// Check if we succeeded until here
	if _, err := os.Stat("privkey.pub"); err == nil {
		fmt.Println("Could not delete privkey.pub, exiting")
		os.Exit(1)
	}

	// Delete unencrypted private key
	os.Remove("secret")

	// Check if we succeeded until here
	if _, err := os.Stat("secret"); err == nil {
		fmt.Println("Could not delete secret, exiting")
		os.Exit(1)
	}

	// Delete unencrypted private key
	os.Remove("privkey")

	// Check if we succeeded until here
	if _, err := os.Stat("privkey"); err == nil {
		fmt.Println("Could not delete unencrypted private key, exiting")
		os.Exit(1)
	}

	// Ask user for GitHub token
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("A GitHub token is needed. Get it from")
	fmt.Println("https://github.com/settings/tokens")
	fmt.Print("Enter GitHub token: ")
	text, _ := reader.ReadString('\n')
	token := strings.TrimSpace(text)
	// fmt.Println(token)

	fmt.Println("Repositories can be on travis.com or on travis.org")
	fmt.Println("Is your repository on travis.com? (yes/no)")
	var client *travis.Client
	if AskForConfirmation() == true {
		fmt.Println("Assuming your repository is on travis.com")
		client = travis.NewClient(travis.ApiComUrl, "")
	} else {
		fmt.Println("Assuming your repository is on travis.org")
		client = travis.NewClient(travis.ApiOrgUrl, "")
	}

	_, _, err = client.Authentication.UsingGithubToken(context.Background(), token)

	// Read existing environment variables on Travis CI
	esList, resp, err := client.EnvVars.ListByRepoSlug(context.Background(), repo_slug)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		fmt.Printf("Could not read existing environment variables on Travis CI for repo %s: invalid http status: %s", repo_slug, resp.Status)
	}

	var existingVars []string
	for _, e := range esList {
		// fmt.Println("* Name:", *e.Name, "Public:", *e.Public, "Id:", *e.Id)
		existingVars = append(existingVars, string(*e.Name))
	}
	// fmt.Println(existingVars)

	SetTravisEnv(client, repo_slug, existingVars, "GITHUB_TOKEN", token)
	SetTravisEnv(client, repo_slug, existingVars, "super_secret_password", super_secret_password)
	// SetTravisEnv(client, repo_slug, existingVars, "FOO", "BAR")

	// TODO: Can we automate this?
	fmt.Println("")
	fmt.Println("Your super secret password is:")
	fmt.Println(super_secret_password)
	fmt.Println("Store it in a safe location, along with your encrypted private key in privkey.enc.")
	fmt.Println("")
	fmt.Println("You can decrypt it with:")
	fmt.Println("openssl aes-256-cbc -pass \"pass:" + super_secret_password + "\" -in privkey.enc -out privkey -d -a")
	fmt.Println("")
	fmt.Println("Please add to your .travis.yml file:")
	fmt.Println("----------------------------------------------------------------------------------------------")
	fmt.Println("before_install:")
	fmt.Println("  - openssl aes-256-cbc -pass \"pass:$super_secret_password\" -in privkey.enc -out privkey -d -a")
	fmt.Println("----------------------------------------------------------------------------------------------")
}

// SetTravisEnv sets a private variable on Travis CI
func SetTravisEnv(client *travis.Client, repo_slug string, existingVars []string, name string, value string) {
	body := travis.EnvVarBody{Name: name, Value: value, Public: false}
	if Contains(existingVars, name) == false {
		fmt.Println("Set environment variable", name, "on Travis CI...")
		_, resp, err := client.EnvVars.CreateByRepoSlug(context.Background(), repo_slug, &body)
		fmt.Println(resp.Status)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Println("Environment variable", name, "already exists on Travis CI, not changing it")
	}
}

// Contains answers whether a []string contains a string
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

//////////////////////////////// start AskForConfirmation

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
// From https://gist.github.com/albrow/5882501#file-confirm-go-L9
func AskForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		fmt.Println(err)
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation()
	}
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

//////////////////////////////// end AskForConfirmation
