// zeroconf.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/grandcat/zeroconf"
)

const zeroconfServiceType = "_appimaged._tcp"

// Announce appimaged on the local network with Zeroconf
func registerZeroconfService() {
	service, err := zeroconf.Register(
		"appimaged",         // service instance name
		zeroconfServiceType, // service type and protocol
		"local.",            // service domain
		88214,               // service port
		nil,                 // service metadata
		nil,                 // register on all network interfaces
	)

	if err != nil {
		log.Fatal(err)
	}

	defer service.Shutdown()

	// Keep this running
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

	log.Println("zeroconf: Shutting down")

	// FIXME: ...and then nothing happens,
	// so we are forcing it... but this feels wrong
	service.Shutdown()
	os.Exit(0)
}

// Browse the local network for Zeroconf services
// TODO: React to services going away,
// https://github.com/grandcat/zeroconf/issues/65
func browseZeroconfServices() {
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("zeroconf: Failed to initialize resolver:", err.Error())
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			log.Println("zeroconf:", entry)
		}
		log.Println("zeroconf: No more entries.")
	}(entries)

	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()
	err = resolver.Browse(ctx, zeroconfServiceType, "local.", entries)
	if err != nil {
		log.Println("zeroconf: Failed to browse:", err.Error())
	}

	<-ctx.Done()

}
