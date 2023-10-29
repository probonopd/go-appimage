package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/probonopd/go-appimage/internal/helpers"
)

// map[path]AppImage. The key should always be an absolute path.
var integrations = make(map[string]*AppImage)

// Most integration tasks are going to be called asyncronously, which could get messy.
// Use this lock to make sure we are only doing 1 integration task at a time.
// If this also get's to messy, we can change to a buffered channel system.
var integrationLock sync.Mutex

func AddIntegration(path string, notify bool) (err error) {
	integrationLock.Lock()
	defer integrationLock.Unlock()
	if _, ok := integrations[path]; ok {
		return
	}
	ai, err := NewAppImage(path)
	if err != nil {
		return
	}
	err = ai._integrate()
	if err != nil {
		// If integration fails, remove any files that might or might not have been integrated by unintegrating.
		helpers.LogError("add integration", err)
		ai._unintegrate()
		return err
	}
	integrations[path] = ai
	if notify {
		sendDesktopNotification("Added "+ai.Name, path, 5000)
	}
	updateChannel <- struct{}{}
	return
}

func RemoveIntegration(path string, notify bool) {
	integrationLock.Lock()
	defer integrationLock.Unlock()
	ai, ok := integrations[path]
	if !ok {
		return
	}
	ai._unintegrate()
	delete(integrations, path)
	if notify {
		sendDesktopNotification("Removed "+ai.Name, path, 5000)
	}
	updateChannel <- struct{}{}
}

func RemoveIntegrationsFromDir(dir string) {
	integrationLock.Lock()
	defer integrationLock.Unlock()
	removed := 0
	for key, ai := range integrations {
		if strings.HasPrefix(key, dir) {
			ai._unintegrate()
			delete(integrations, key)
			removed++
		}
	}
	if removed > 0 {
		sendDesktopNotification("Removed "+strconv.Itoa(removed)+" applications from "+dir, "", 5000)
		updateChannel <- struct{}{}
	}
}

func AddIntegrationsFromDir(dir string) {
	integrationLock.Lock()
	defer integrationLock.Unlock()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	added := 0
	for _, fil := range ents {
		path := filepath.Join(dir, fil.Name())
		if IsPossibleAppImage(path) {
			_, ok := integrations[path]
			if ok {
				continue
			}
			ai, err := NewAppImage(path)
			if err != nil {
				return
			}
			err = ai._integrate()
			if err != nil {
				// If integration fails, remove any files that might or might not have been integrated by unintegrating.
				helpers.LogError("add integration", err)
				ai._unintegrate()
				continue
			}
			integrations[path] = ai
			added++
		}
	}
	if added > 0 {
		sendDesktopNotification("Added "+strconv.Itoa(added)+" applications from "+dir, "", 5000)
		updateChannel <- struct{}{}
	}
}
