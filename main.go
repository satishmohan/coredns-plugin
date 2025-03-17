package main

import (
	"log"

	"github.com/coredns/caddy"
	"github.com/samohan/coredns-plugin/appidentify"
)

func setup(c *caddy.Controller) error { // Corrected type to caddy.Controller
	_, err := appidentify.Setup("appidentify/applications.json") // Updated path
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
