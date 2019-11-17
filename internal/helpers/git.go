// https://git-scm.com/book/en/v2/Appendix-B%3A-Embedding-Git-in-your-Applications-go-git

package helpers

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/src-d/go-git.v4"
)

func TestGit() {

	dir := "/tmp/foo"

	s, _ := os.Stat(dir)

	var r *git.Repository
	var err error
	if s.IsDir() == false {
		r, err = git.PlainClone(dir, false, &git.CloneOptions{
			URL:      "https://github.com/src-d/go-git",
			Progress: os.Stdout,
		})
		if err != nil {
			fmt.Println("git.PlainClone:", err)
		}
	} else {
		r, err = git.PlainOpen(dir)
		if err != nil {
			fmt.Println("git.PlainOpen:", err)
		}
	}

	if r == nil {
		fmt.Println("Could not open repository", dir)
		os.Exit(1)
	}
	fmt.Println(r)

	// retrieves the branch pointed by HEAD
	ref, err := r.Head()
	if err != nil {
		fmt.Println("r.Head:", err)
	}

	fmt.Println(ref)

	// get the commit object, pointed by ref
	commit, _ := r.CommitObject(ref.Hash())
	fmt.Println(commit.Message)
	// retrieves the commit history
	//	history, err := commit.History()

	// iterates over the commits and print each
	// for _, c := range history {
	// 	fmt.Println(c)
	// }

}

// GetGitRepository returns a git Repository if cwd is a git repository, and error otherwise
func GetGitRepository() (*git.Repository, error) {
	var repo *git.Repository
	cwd, err := os.Getwd()
	if err != nil {
		return repo, err
	}

	repo, err = git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return repo, err
	}

	if repo == nil {
		fmt.Println()
		return repo, errors.New("Could not open repository. Please execute this command from within a git repository. " + cwd)
	}

	return repo, nil
}
