package goappimage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
)

//NOTE: If you internet is a bit slow, it's not a bad idea to download it manually instead of letting the test download it
//TODO: Change to a different AppImage since Blender is a bit large...
const type1TestURL = "https://bintray.com/probono/AppImages/download_file?file_path=Blender-2.78c.glibc2.17-x86_64.AppImage"
const type1TestFilename = "Blender-2.78c.glibc2.17-x86_64.AppImage"

//NOTE: If you internet is a bit slow, it's not a bad idea to download it manually instead of letting the test download it
const type2TestURL = "https://github.com/subsurface/subsurface/releases/download/v4.9.4/Subsurface-4.9.4-x86_64.AppImage"
const type2TestFilename = "Subsurface-4.9.4-x86_64.AppImage"

func TestAppImageType1(t *testing.T) {
	testImg := getAppImage(1, t)
	ai, err := NewAppImage(testImg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Name", ai.Name)
	t.Fatal("No Problem")
}

func TestAppImageType2(t *testing.T) {
	testImg := getAppImage(2, t)
	ai, err := NewAppImage(testImg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Name", ai.Name)
	t.Fatal("No Problem")
}

func getAppImage(imageType int, t *testing.T) string {
	// var url string
	var filename string
	switch imageType {
	case 1:
		filename = type1TestFilename
	case 2:
		filename = type2TestFilename
	default:
		t.Fatal("What are you doing here?")
	}
	wdDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Open(wdDir + "/testing")
	if os.IsNotExist(err) {
		err = os.Mkdir(wdDir+"/testing", os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}
		_, err = os.Open(wdDir + "/testing")
		if err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	_, err = os.Open(wdDir + "/testing/" + filename)
	if os.IsNotExist(err) {
		downloadTestImage(imageType, wdDir+"/testing", t)
		_, err = os.Open(wdDir + "/testing/" + filename)
		if err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	return wdDir + "/testing/" + filename
}

func downloadTestImage(imageType int, dir string, t *testing.T) {
	var filename string
	var url string
	switch imageType {
	case 1:
		url = type1TestURL
		filename = type1TestFilename
	case 2:
		url = type2TestURL
		filename = type2TestFilename
	default:
		t.Fatal("What are you doing here?")
	}
	appImage, err := os.Create(dir + "/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	defer appImage.Close()
	check := http.Client{
		CheckRedirect: func(r *http.Request, _ []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	resp, err := check.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(appImage, resp.Body)
	if err != nil {
		t.Fatal(err)
	}
}
