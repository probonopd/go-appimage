// zeroconf.go
package main

import (
	"context"
	"log"
	"net"
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
		log.Println("Error registering zeroconf service")
		return
	}

	defer service.Shutdown()

	// Keep this running
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig

	log.Println("zeroconf: Shutting down...")

	// FIXME: ...and then nothing happens,
	// so we are forcing it... but this feels wrong
	service.Shutdown()
	log.Println("zeroconf: Shut down")
	os.Exit(0)
}

// Browse the local network for Zeroconf services
// TODO: React to services going away,
// https://github.com/grandcat/zeroconf/issues/65
func browseZeroconfServices() {
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Println("zeroconf: Failed to initialize resolver:", err)
		return
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
		log.Println("zeroconf: Failed to browse:", err)
	}

	<-ctx.Done()

}

// CheckIfConnectedToNetwork returns true if connected to a network
func CheckIfConnectedToNetwork() bool {
	var interfaces []net.Interface
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	if len(ifaces) == 0 {
		return false
	}
	for _, ifi := range ifaces {
		if (ifi.Flags & net.FlagUp) == 0 {
			continue
		}
		if (ifi.Flags & net.FlagMulticast) > 0 {
			interfaces = append(interfaces, ifi)
		}
	}

	var AddrIPv4 []net.IP
	var AddrIPv6 []net.IP
	for _, iface := range interfaces {
		v4, v6 := addrsForInterface(&iface)
		AddrIPv4 = append(AddrIPv4, v4...)
		AddrIPv6 = append(AddrIPv6, v6...)
	}

	if AddrIPv4 == nil && AddrIPv6 == nil {
		return false
	} else {
		return true
	}

}

func addrsForInterface(iface *net.Interface) ([]net.IP, []net.IP) {
	var v4, v6, v6local []net.IP
	addrs, _ := iface.Addrs()
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				v4 = append(v4, ipnet.IP)
			} else {
				switch ip := ipnet.IP.To16(); ip != nil {
				case ip.IsGlobalUnicast():
					v6 = append(v6, ipnet.IP)
				case ip.IsLinkLocalUnicast():
					v6local = append(v6local, ipnet.IP)
				}
			}
		}
	}
	if len(v6) == 0 {
		v6 = v6local
	}
	return v4, v6
}
