// TODO:
// Make it secure. Currently anyone can post anything to those topics
// which means that the following things could happen:
// * Update checks might not be triggered if someone publishes wrong versions
// * Unnecessary update checks might be triggered if someone publishes wrong versions
//   but the updater would see that no update is needed
// The solution is using a private MQTT broker on which we can limit who is allowed to
// publish to which topics. To be checked how to make this authorization
// most seamless for GitHub, OBS,... users so that ideally they have
// no manual work at all. Do these platforms have some callback functions that we could use?
// Option 1: We let users' appimagetool write to the MQTT topics they have access to
// Option 2: Users do not write to MQTT topics at all, only AppImageHub does
// E.g., appimagetool could upload, then ping AppImageHub and then AppImageHub could
// do its checks including the update mechanism,
// and only if this passes AppImageHub would publish to the topic

// TODO: Replace by IPFS PubSub?
// Could that also ensure that only permitted users
// can publish to their channel?

// To test:
// Go to http://www.hivemq.com/demos/websocket-client/
// Publish to topic
// p9q358t/github.com/probonopd/appimage/continuous/version
// p9q358t/gh-releases-zsync%7CAppImage%7CAppImageKit%7Ccontinuous%7Cappimagetool-x86_64.AppImage.zsync/version
// p9q358t/gh-releases-zsync%7Cprobonopd%7Cmerkaartor%7Ccontinuous%7CMerkaartor%2A-x86_64.AppImage.zsync/version
// a message with the current $VERSION string for the continuous build
// and Retain enabled
//
// TODO: Perhaps use the SHA1 hash from the zsync file to match files, rather than just a version string;
// in case the version string is the same but the files are different?
// But then we would need to either have or calculate that hash inside the AppImage.
// Is this what the digest is for in https://github.com/AppImage/AppImageSpec/issues/29?

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/adrg/xdg"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	helpers "github.com/probonopd/appimage/internal/helpers"
	"gopkg.in/ini.v1"
)

func connect(clientId string, uri *url.URL) mqtt.Client {
	opts := createClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}

func createClientOptions(clientId string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientId)
	return opts
}

func SubscribeMQTT(client mqtt.Client, updateinformation string) {
	prefix := "p9q358t" // Our namespace
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	topic := prefix + "/" + queryEscapedUpdateInformation + "/#"
	fmt.Println("mqtt: Subscribing for", updateinformation)
	fmt.Println("mqtt: Waiting for messages on topic", prefix+"/"+queryEscapedUpdateInformation+"/version")
	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		// fmt.Printf("* [%s] %s\n", msg.Topic(), string(msg.Payload()))
		// fmt.Println(topic)
		short := strings.Replace(msg.Topic(), prefix+"/", "", -1)
		parts := strings.Split(short, "/")
		fmt.Println("mqtt: received:", parts)
		if len(parts) < 2 {
			return
		}
		if parts[1] == "version" {
			version := string(msg.Payload())
			queryEscapedUpdateInformation := parts[0]
			fmt.Println("mqtt:", queryEscapedUpdateInformation, "reports version", version)
			unescapedui, _ := url.QueryUnescape(queryEscapedUpdateInformation)
			results := FindAppImagesWithMatchingUpdateInformation(unescapedui)
			fmt.Println("mqtt:", updateinformation, "reports version", version, "we have matching", results)
			// Find the most recent local file, based on https://stackoverflow.com/a/45579190
			mostRecent := helpers.FindMostRecentFile(results)
			fmt.Println("mqtt:", updateinformation, "reports version", version, "we have matching", mostRecent)
			// TODO: If version the AppImage has embededed is different, if yes launch AppImageUpdate
			if helpers.IsCommandAvailable("AppImageUpdate") {
				fmt.Println("mqtt: AppImageUpdate", mostRecent)
				cmd := exec.Command("AppImageUpdate", mostRecent)
				log.Printf("Running AppImageUpdate command and waiting for it to finish...")
				err := cmd.Run()
				log.Printf("AppImageUpdate finished with error: %v", err)
			}
		}
	})
}

// FindAppImagesWithMatchingUpdateInformation finds registered AppImages
// that have matching upate information embedded
// FIXME: Take care of things like "appimaged wrap" or "firejail" prefixes. We need to do this differently!
func FindAppImagesWithMatchingUpdateInformation(updateinformation string) []string {
	files, err := ioutil.ReadDir(xdg.DataHome + "/applications/")
	helpers.LogError("desktop", err)
	var results []string
	if err != nil {
		return results
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".desktop") && strings.HasPrefix(file.Name(), "appimagekit_") {
			cfg, e := ini.Load(xdg.DataHome + "/applications/" + file.Name())
			helpers.LogError("desktop", e)
			dst := cfg.Section("Desktop Entry").Key(ExecLocationKey).String()
			_, err = os.Stat(dst)
			if os.IsNotExist(err) {
				log.Println(dst, "does not exist, it is mentioned in", xdg.DataHome+"/applications/"+file.Name())
				continue
			}
			ai := newAppImage(dst)
			ui, err := ai.ReadUpdateInformation()
			if err == nil && ui != "" {
				//log.Println("updateinformation:", ui)
				// log.Println("updateinformation:", url.QueryEscape(ui))
				unescapedui, _ := url.QueryUnescape(ui)
				// log.Println("updateinformation:", unescapedui)
				if updateinformation == unescapedui {
					results = append(results, ai.path)
				}
			}

			continue
		}
	}
	return results
}
