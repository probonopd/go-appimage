// Publish MQTT messages
// Based on
// https://github.com/CloudMQTT/go-mqtt-example/blob/master/main.go

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
// p9q358t/github.com/probonopd/go-appimage/continuous/version
// p9q358t/gh-releases-zsync%7CAppImage%7CAppImageKit%7Ccontinuous%7Cappimagetool-x86_64.AppImage.zsync/version
// p9q358t/gh-releases-zsync%7Cprobonopd%7Cmerkaartor%7Ccontinuous%7CMerkaartor%2A-x86_64.AppImage.zsync/version
// a message with the current $VERSION string for the continuous build
// and Retain enabled
//
// TODO: Perhaps use the SHA1 hash from the zsync file to match files, rather than just a version string;
// in case the version string is the same but the files are different?
// But then we would need to either have or calculate that hash inside the AppImage.
// Is this what the digest is for in https://github.com/AppImage/AppImageSpec/issues/29?

package helpers

import (
	"fmt"
	"log"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Define the PubSubData that gets exchanged between
// AppImage authoring tool and AppImage desktop integration tool
// (e.g., appimagetool and appimaged).
// For best practices regarding JSON payloads, see
// https://yourbasic.org/golang/json-example/
// e.g., use int64 for number.
// We try to keep the messages as small as possible and
// not transmit redundant information here that can be constructed
// by the client directly (e.g., the URL to the GitHub repository
// can logially be derived from the updateinformation string)
type PubSubData struct {
	Name    string
	Version string
	// Fruit   []string
	// Size int64 // FIXME: Size is not sufficient to tell two AppImages apart since squashfs pads the end
	// Id      int64  `json:"ref"` // A field can be encoded to a different key name in JSON
	// private string // An unexported (lowercase) field is not encoded
	FSTime time.Time // Use this format and time.Now() to transport timestamps. We (ab)use FSTime to check local files with "unsquashfs -fstime" to see whether they are "different"
}

// TODO: Instead of using hardcoded values here, we should grab those values from
// some URL where we can change them, to allow the broker and namespace to be
// switched at any time without having to update all clients.
// We should also publish the version of the message format
// there so that we could force applications to be updated if they are not built
// for that version of the message format
const MQTTServerURI = "http://broker.hivemq.com:1883"
const MQTTNamespace = "p9q358t" // Our namespace. Our topic begins with this

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

func PublishMQTTMessage(updateinformation string, version string) {
	uri, err := url.Parse(MQTTServerURI)
	if err != nil {
		log.Fatal(err)
	}
	client := connect("pub", uri)
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	topic := MQTTNamespace + "/" + queryEscapedUpdateInformation + "/version" // TODO: Publish hash instead of or in addition to version
	fmt.Println("Publishing version", version, "for", updateinformation)
	token := client.Publish(topic, 2, true, version) // Retain; QoS 2
	token.Wait()
}
