package main

import (
	"log"

	"github.com/coredns/coredns/plugin"
	"github.com/samohan/coredns-plugin/appidentify"
)

func setup(c *plugin.Controller) error {
	plugin, err := appidentify.Setup("applications.json")
	if err != nil {
		return err
	}

	c.Next = plugin
	return nil
}

func main() {
	log.Println("CoreDNS plugin initialized")
}
