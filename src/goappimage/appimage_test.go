package goappimage

import (
	"os"
	"testing"
)

const type1TestURL = "https://bintray.com/probono/AppImages/download_file?file_path=Audacity-2.1.2.glibc2.15-x86_64.AppImage"
const type1TestFilename = "Audacity-2.1.2.glibc2.15-x86_64.AppImage"

func TestAppImageType1(t *testing.T) {
	testImg := getAppImage(1, t)
	ai := NewAppImage(testImg)
	if ai.imageType == -1 {
		t.Fatal("Not an appimage")
	}
	_, err := ai.ExtractFileReader("*.desktop", false)
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
		//TODO: download stuff
	} else if err != nil {
		t.Fatal(err)
	}
	return wdDir + "/testing/" + filename
}

func downloadTestImage(imageType int, t *testing.T) {
}
