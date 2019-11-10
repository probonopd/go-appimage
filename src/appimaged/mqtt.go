package main

import (
	"encoding/json"
	"fmt"

	"log"
	"net/url"

	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	helpers "github.com/probonopd/appimage/internal/helpers"
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
	time.Sleep(time.Second * 60) // We get retained messages immediately when we subscribe;
	// at this point our AppImage may not be integrated yet...
	// Also it's better user experience not to be bombarded with updates immediately at startup.
	// 60 seconds should be plenty of time.
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	topic := helpers.MQTTNamespace + "/" + queryEscapedUpdateInformation + "/#"
	fmt.Println("mqtt: Subscribing for", updateinformation)
	fmt.Println("mqtt: Waiting for messages on topic", helpers.MQTTNamespace+"/"+queryEscapedUpdateInformation+"/version")

	client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		// fmt.Printf("* [%s] %s\n", msg.Topic(), string(msg.Payload()))
		// fmt.Println(topic)
		short := strings.Replace(msg.Topic(), helpers.MQTTNamespace+"/", "", -1)
		parts := strings.Split(short, "/")
		fmt.Println("mqtt: received:", parts)
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
			fmt.Println("mqtt:", queryEscapedUpdateInformation, "reports version", version)
			unescapedui, _ := url.QueryUnescape(queryEscapedUpdateInformation)
			if unescapedui == thisai.updateinformation {
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				log.Println("+ Update available for this AppImage.")
				log.Println("+ Something special should happen here: Selfupdate")
				log.Println("+ To be imlpemented.")
				log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++")
				SimpleNotify("Update available", "An update for the AppImage daemon is available; I could update myself now...", 0)
			}

			mostRecent := FindMostRecentAppImageWithMatchingUpdateInformation(unescapedui)
			ai := NewAppImage(mostRecent)

			fstime := ai.getFSTime()
			fmt.Println("mqtt:", updateinformation, "reports version", version, "we have matching", mostRecent, "with FSTime", fstime)

			// FIXME: Only notify if the version is newer than what we already have.
			// More precisely, if the AppImage being offered is different from the one we already have
			// even despite version numbers being the same.
			// Blocked by https://github.com/AppImage/AppImageSpec/issues/29

			if fstime != data.FSTime {

				// TODO: Do some checks before, e.g., see whether we already have it,
				// and whether it is really available for download, and which version the existing
				// AppImage claims to be

				SimpleNotify("Update available", ai.niceName+"\ncan be updated to version "+version, 120000)
			}
		}
	})
}
