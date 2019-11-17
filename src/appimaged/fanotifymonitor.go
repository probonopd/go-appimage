// Watches the filesystem for accesses similar to what an on-demand virus scanner would do
// Heavily borrowed from
// https://github.com/coolhacks/docker-hacks/blob/d7ea13522188233d5e8a97179d2b0a872239f58d/examples/docker-slim/src/launcher/main.go
// This needs root permissions.
// FIXME: Blocked by: How can I get root permissions from polkit for this?

// FIXME: This builds on amd64 but not on arm64, getting:
// fanotifymonitor.go:111:10: nd.Mark undefined (type *fanotify.NotifyFD has no field or method Mark)
// https://travis-ci.com/probonopd/go-appimage/jobs/257232286
// Hence the code is commented out for now.

package main

/*

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/amenzhinsky/go-polkit"

	"github.com/cloudimmunity/pdiscover"
	helpers "github.com/probonopd/appimage/internal/helpers"
	"github.com/s3rj1k/go-fanotify/fanotify"
)

// FANotifyMonitor watches the local filesystem for accesses using fanotify
func FANotifyMonitor() {

	// Get permission from polkit
	authority, err := polkit.NewAuthority()
	if err != nil {
		helpers.PrintError("fanotifymonitor: polkit.NewAuthority", err)
	}
	action := "org.gtk.vfs.file-operations-helper"
	// Why this one? Because
	// sudo grep -r "sudo" /usr/share/polkit-1/
	// indicates that it is granted when the user is in the sudoers group
    // FIXME: Still getting:
	// polkit: Is authorized: true org.gtk.vfs.file-operations-helper
	// launcher: args => []string{"/tmp/___appimaged"}
	// launcher: fanotifymonitor starting...
	// fanotifymonitor: listen_events start
	// fanotifymonitor error: operation not permitted

	// FIXME: Looks like there is no action for fanotify
	// Can we get root directly, e.g., using auth_admin_keep without a /etc/polkit-1/rules.d/51-pkexec-auth-admin-keep.rules file?
	// Alternatively, can we get permission from polkit to write that file there?
	result, err := authority.CheckAuthorization(action, nil, polkit.CheckAuthorizationAllowUserInteraction, "")
	if err != nil {
		helpers.PrintError("fanotifymonitor: polkit.CheckAuthorization", err)
	}

	log.Printf("polkit: Is authorized: %t %s\n", result.IsAuthorized, action)

	// Run the fanotifymonitor
	log.Printf("launcher: args => %#v\n", os.Args)
	pidsChan := make(chan []int, 1)
	fanotifymonitor(nil, nil, pidsChan)

	// Run forever
	c := make(chan struct{})
	<-c

}

func failOnError(err error) {
	if err != nil {
		log.Fatalln("launcher: ERROR =>", err)
	}
}

func failWhen(cond bool, msg string) {
	if cond {
		log.Fatalln("launcher: ERROR =>", msg)
	}
}

func myFileDir() string {
	dirName, err := filepath.Abs(filepath.Dir(os.Args[0]))
	failOnError(err)
	return dirName
}

func fileDir(fileName string) string {
	dirName, err := filepath.Abs(filepath.Dir(fileName))
	failOnError(err)
	return dirName
}

func sendPids(pidList []int) {
	pidsData, err := json.Marshal(pidList)
	failOnError(err)

	fanotifymonitorSocket, err := net.Dial("unix", "/tmp/afanotifymonitor.sock")
	failOnError(err)
	defer fanotifymonitorSocket.Close()

	fanotifymonitorSocket.Write(pidsData)
	fanotifymonitorSocket.Write([]byte("\n"))
}

/////////

type event struct {
	Pid  int32
	File string
}

func check(err error) {
	if err != nil {
		log.Fatalln("fanotifymonitor error:", err)
	}
}

func listen_events(mount_point string, stop chan bool) chan map[event]bool {
	log.Println("fanotifymonitor: listen_events start")

	nd, err := fanotify.Initialize(fanotify.FAN_CLASS_NOTIF, os.O_RDONLY)
	check(err)
	err = nd.Mark(fanotify.FAN_MARK_ADD|fanotify.FAN_MARK_MOUNT, fanotify.FAN_ACCESS|fanotify.FAN_OPEN, -1, mount_point)
	check(err)

	events_chan := make(chan map[event]bool, 1)

	go func() {
		log.Println("fanotifymonitor: listen_events worker starting")
		events := make(map[event]bool, 1)
		eventChan := make(chan event)
		go func() {
			for {
				data, err := nd.GetEvent()
				check(err)
				path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", data.File.Fd()))
				check(err)
				e := event{data.PID, path}
				data.File.Close()
				eventChan <- e
			}
		}()

		s := false
		for !s {
			select {
			case <-time.After(20 * time.Second):
				log.Println("fanotifymonitor: listen_events - event timeout...")
				s = true
			case <-stop:
				log.Println("fanotifymonitor: listen_events stopping...")
				s = true
			case e := <-eventChan:
				events[e] = true
				log.Printf("fanotifymonitor: listen_events event => %#v\n", e)
			}
		}

		log.Printf("fanotifymonitor: listen_events sending %v events...\n", len(events))
		events_chan <- events
	}()

	return events_chan
}

func fanotifymonitor_process(stop chan bool) chan map[int][]int {
	log.Println("fanotifymonitor: fanotifymonitor_process start")

	watcher, err := pdiscover.NewAllWatcher(pdiscover.PROC_EVENT_ALL)
	check(err)

	forks_chan := make(chan map[int][]int, 1)

	go func() {
		forks := make(map[int][]int)
		s := false
		for !s {
			select {
			case <-stop:
				s = true
			case ev := <-watcher.Fork:
				forks[ev.ParentPid] = append(forks[ev.ParentPid], ev.ChildPid)
			case <-watcher.Exec:
			case <-watcher.Exit:
			case err := <-watcher.Error:
				log.Println("error: ", err)
				panic(err)
			}
		}
		forks_chan <- forks
		watcher.Close()
	}()

	return forks_chan
}

func get_files(events chan map[event]bool, pids_map chan map[int][]int, pids chan []int) []string {
	p := <-pids
	pm := <-pids_map
	e := <-events
	all_pids := make(map[int]bool, 0)

	for _, v := range p {
		all_pids[v] = true
		for _, pl := range pm[v] {
			all_pids[pl] = true
		}
	}

	files := make([]string, 0)
	for k, _ := range e {
		_, found := all_pids[int(k.Pid)]
		if found {
			files = append(files, k.File)
		}
	}
	return files
}

func get_files_all(events chan map[event]bool) []string {
	log.Println("launcher: get_files_all - getting events...")
	e := <-events
	log.Println("launcher: get_files_all - event count =>", len(e))
	files := make([]string, 0)
	for k, _ := range e {
		log.Println("launcher: get_files_all - adding file =>", k.File)
		files = append(files, k.File)
	}
	return files
}

func files_to_inodes(files []string) []int {
	cmd := "/usr/bin/stat"
	args := []string{"-L", "-c", "%i"}
	args = append(args, files...)
	inodes := make([]int, 0)

	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()
	for _, i := range strings.Split(string(out), "\n") {
		inode, err := strconv.Atoi(strings.TrimSpace(i))
		if err != nil {
			continue
		}
		inodes = append(inodes, inode)
	}
	return inodes
}

func find_symlinks(files []string, mp string) map[string]bool {
	cmd := "/usr/bin/find"
	args := []string{"-L", mp, "-mount", "-printf", "%i %p\n"}
	c := exec.Command(cmd, args...)
	out, _ := c.Output()
	c.Wait()

	inodes := files_to_inodes(files)
	inode_to_files := make(map[int][]string)

	for _, v := range strings.Split(string(out), "\n") {
		v = strings.TrimSpace(v)
		info := strings.Split(v, " ")
		inode, err := strconv.Atoi(info[0])
		if err != nil {
			continue
		}
		inode_to_files[inode] = append(inode_to_files[inode], info[1])
	}

	result := make(map[string]bool, 0)
	for _, i := range inodes {
		v := inode_to_files[i]
		for _, f := range v {
			result[f] = true
		}
	}
	return result
}

func cp(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		log.Println("launcher: fanotifymonitor - cp - error opening source file =>", src)
		return err
	}
	defer s.Close()

	dstDir := fileDir(dst)
	err = os.MkdirAll(dstDir, 0777)
	if err != nil {
		log.Println("launcher: fanotifymonitor - dir error =>", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		log.Println("launcher: fanotifymonitor - cp - error opening dst file =>", dst)
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func fanotifymonitor(stop_work chan bool, stop_work_ack chan bool, pids chan []int) {
	log.Println("launcher: fanotifymonitor starting...")
	mount_point := "/"

	stop_events := make(chan bool, 1)
	_ = listen_events(mount_point, stop_events)

	//stop_process := make(chan bool, 1)
	//pids_map := fanotifymonitor_process(stop_process)

	go func() {
		log.Println("launcher: fanotifymonitor - waiting to stop fanotifymonitoring...")
		<-stop_work
		log.Println("launcher: fanotifymonitor - stop message...")
		stop_events <- true

		stop_work_ack <- true
	}()
}

*/
