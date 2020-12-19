package goappimage

import (
	"fmt"
	"os"
	"testing"
)

const type1TestURL = "https://bintray.com/probono/AppImages/download_file?file_path=Blender-2.78c.glibc2.17-x86_64.AppImage"
const type1TestFilename = "Blender-2.78c.glibc2.17-x86_64.AppImage"

func TestAppImageType1(t *testing.T) {
	wdDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	//make sure we have a testing dir
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
	testImg := getAppImage(1, t)
	ai, err := NewAppImage(testImg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("name", ai.Name)
	if ai.imageType == -1 {
		t.Fatal("Not an appimage")
	}
	_, err = newType1Reader(testImg)
	if err != nil {
		t.Fatal(err)
	}
	t.Fatal("No Problem")
}

func getAppImage(imageType int, t *testing.T) string {
	// var url string
	var filename string
	switch imageType {
	case 1:
		// url = type1TestURL
		filename = type1TestFilename
	default:
		t.Fatal("What are you doing here?")
	}
	wdDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	//make sure we have a testing dir
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
		downloadTestImage(imageType, t)
		_, err = os.Open(wdDir + "/testing/" + filename)
		if err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	return wdDir + "/testing/" + filename
}

func downloadTestImage(imageType int, t *testing.T) {
}
