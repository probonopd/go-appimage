package main

import (
	"encoding/json"
	"fmt"

	"log"
	"net/url"

	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/probonopd/go-appimage/internal/helpers"
)

func connect(clientId string, uri *url.URL) mqtt.Client {
	opts := createClientOptions(clientId, uri)
	client := mqtt.NewClient(opts)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		helpers.PrintError("MQTT", err) // We land here in "horror network" situations, so this must not be fatal
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

// UnSubscribeMQTT unubscribe from receiving update notifications for updateinformation
// TODO: Keep track of what we have already subscribed, and remove from that list
func UnSubscribeMQTT(client mqtt.Client, updateinformation string) {
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	client.Unsubscribe(queryEscapedUpdateInformation)
}

// SubscribeMQTT subscribes to receive update notifications for updateinformation
// TODO: Keep track of what we have already subscribed, and don't subscribe again
func SubscribeMQTT(client mqtt.Client, updateinformation string) {

	if helpers.SliceContains(subscribedMQTTTopics, updateinformation) == true {
		// We have already subscribed to this; so nothing to do here
		return
	} else {
		// Need to do this immediately here, otherwise it comes too late
		subscribedMQTTTopics = helpers.AppendIfMissing(subscribedMQTTTopics, updateinformation)
	}
	time.Sleep(time.Second * 10) // We get retained messages immediately when we subscribe;
	// at this point our AppImage may not be integrated yet...
	// Also it's better user experience not to be bombarded with updates immediately at startup.
	// 10 seconds should be plenty of time.
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	topic := helpers.MQTTNamespace + "/" + queryEscapedUpdateInformation + "/#"

	if *verbosePtr == true {
		log.Println("mqtt: Waiting for messages on topic", helpers.MQTTNamespace+"/"+queryEscapedUpdateInformation+"/version")
	} else {
		log.Println("Subscribing to updates for", updateinformation)
	}
	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		// log.Printf("* [%s] %s\n", msg.Topic(), string(msg.Payload()))
		// log.Println(topic)
		short := strings.Replace(msg.Topic(), helpers.MQTTNamespace+"/", "", -1)
		parts := strings.Split(short, "/")
		log.Println("mqtt: received:", parts)
		if len(parts) < 2 {
			return
		}

		if parts[1] == "version" {
			// version := string(msg.Payload())
			// Decode incoming JSON
			var data helpers.PubSubData
			err := json.Unmarshal(msg.Payload(), &data)
			if err != nil {
				helpers.PrintError("mqtt unmarshal", err)
			}
			version := data.Version
			if version == "" {
				return
			}
			queryEscapedUpdateInformation := parts[0]
			log.Println("mqtt:", queryEscapedUpdateInformation, "reports version", version)
			unescapedui, _ := url.QueryUnescape(queryEscapedUpdateInformation)
			if unescapedui == thisai.updateinformation {
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				log.Println("+ Update available for this AppImage.")
				log.Println("+ Something special should happen here: Selfupdate")
				log.Println("+ To be imlpemented.")
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				sendDesktopNotification("Update available", "An update for the AppImage daemon is available; I could update myself now...", 0)
			}

			mostRecent := FindMostRecentAppImageWithMatchingUpdateInformation(unescapedui)
			ai := NewAppImage(mostRecent)

			fstime := ai.getFSTime()
			log.Println("mqtt:", updateinformation, "reports version", version, "with FSTime", data.FSTime.Unix(), "- we have", mostRecent, "with FSTime", fstime.Unix())

			// FIXME: Only notify if the version is newer than what we already have.
			// More precisely, if the AppImage being offered is *different* from the one we already have
			// even despite version numbers being the same.
			// Blocked by https://github.com/AppImage/AppImageSpec/issues/29,
			// in the meantime we are using "-fstime" from unsquashfs to
			// check whether two AppImages are "different". Note that we are
			// not using this to determine whether which one is newer,
			// since we don't trust that timestamp enough.
			// We just determine what is the newest AppImage on the local system
			// and if that one is deemed "different" from what was received over PubPub,
			// then we assume we should offer to update.
			// This mechanism should be more robust against wrong timestamps.
			if fstime.Unix() != data.FSTime.Unix() {
				ui, err := helpers.NewUpdateInformationFromString(updateinformation)
				if err != nil {
					helpers.PrintError("mqtt: NewUpdateInformationFromString:", err)
				} else {
					msg, err := helpers.GetCommitMessageForLatestCommit(ui)
					// changelog_url, _ := helpers.GetReleaseURL(ui)
					if err != nil {
						helpers.PrintError("mqtt: GetCommitMessageForLatestCommit:", err)
					} else {
						// The following could not be tested yet
						go sendUpdateDesktopNotification(ai, version, msg)
						//sendDesktopNotification("Update available for "+ai.niceName, "It can be updated to version "+version+". \n"+msg, 120000)
					}
				}
			} else {
				log.Println("mqtt: Not taking action on", ai.niceName, "because FStime is identical")

			}
		}
	})
}
