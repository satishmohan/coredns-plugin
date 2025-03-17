package main

import (
	"log"

	"github.com/coredns/caddy"
	"github.com/satishmohan/coredns-plugin/appidentify" // Updated to match /v1 module path
)

func setup(c *caddy.Controller) error {
	_, err := appidentify.Setup("appidentify/applications.json")
	if err != nil {
		return err
	}

	// Use plugin.Handler to chain plugins
	c.OnStartup(func() error {
		log.Println("AppIdentify plugin setup complete")
		return nil
	})

	return nil
}

func main() {
	log.Println("CoreDNS plugin initialized")
}
