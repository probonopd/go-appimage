// Publish MQTT messages
// Based on
// https://github.com/CloudMQTT/go-mqtt-example/blob/master/main.go

package main

import (
	"fmt"
	"log"
	"net/url"
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

func PublishMQTTMessage(updateinformation string, version string) {
	uri, err := url.Parse(helpers.MQTTServerURI)
	if err != nil {
		log.Fatal(err)
	}
	client := connect("pub", uri)
	queryEscapedUpdateInformation := url.QueryEscape(updateinformation)
	if queryEscapedUpdateInformation == "" {
		return
	}
	topic := helpers.MQTTNamespace + "/" + queryEscapedUpdateInformation + "/version"
	fmt.Print("Publishing version", version, "for", updateinformation)
	client.Publish(topic, 0, true, version) // Retain
}
