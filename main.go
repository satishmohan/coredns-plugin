package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http" // Added for HTTP server
	"os/exec"
	"sync" // Added for thread-safe access to DetectedIPs

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

type AppIdentifyPlugin struct {
	Next         plugin.Handler
	AppDirectory map[string][]string // Map of application names to domains
	DetectedIPs  map[string]struct{} // Set of detected IPs
	mu           sync.Mutex          // Mutex for thread-safe access to DetectedIPs
}

func (a *AppIdentifyPlugin) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	log.Printf("Intercepted DNS query for: %s", r.Question[0].Name)

	// Check if the query matches any application domain
	for appName, domains := range a.AppDirectory {
		for _, domain := range domains {
			if r.Question[0].Name == domain {
				log.Printf("Detected application: %s", appName)

				// Extract IP addresses from the DNS response
				for _, answer := range r.Answer {
					if aRecord, ok := answer.(*dns.A); ok {
						ip := aRecord.A.String()

						// Thread-safe addition to DetectedIPs
						a.mu.Lock()
						a.DetectedIPs[ip] = struct{}{}
						a.mu.Unlock()

						log.Printf("Added IP %s for application %s", ip, appName)

						// Update the Linux kernel IP set
						if err := addToIPSet(ip); err != nil {
							log.Printf("Failed to add IP %s to IP set: %v", ip, err)
						}
					}
				}
			}
		}
	}

	return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
}

func (a *AppIdentifyPlugin) Name() string {
	return "appidentify"
}

func setup(c *plugin.Controller) error {
	// Load application directory from JSON file
	appDirectory, err := loadAppDirectory("applications.json")
	if err != nil {
		return err
	}

	a := &AppIdentifyPlugin{
		AppDirectory: appDirectory,
		DetectedIPs:  make(map[string]struct{}),
	}

	// Start the HTTP server in a separate goroutine
	go startHTTPServer(a)

	c.Next = a
	return nil
}

func loadAppDirectory(filePath string) (map[string][]string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var appDirectory map[string][]string
	if err := json.Unmarshal(data, &appDirectory); err != nil {
		return nil, err
	}

	return appDirectory, nil
}

// addToIPSet adds an IP address to a Linux kernel IP set using the `ipset` command.
func addToIPSet(ip string) error {
	cmd := exec.Command("ipset", "add", "detected_ips", ip, "-exist") // Use -exist to avoid duplicate errors
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// startHTTPServer starts a simple HTTP server to display detected applications and their IPs.
func startHTTPServer(a *AppIdentifyPlugin) {
	http.HandleFunc("/detected", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		defer a.mu.Unlock()

		// Prepare a map of applications to their detected IPs
		appIPs := make(map[string][]string)
		for appName, domains := range a.AppDirectory {
			for _, domain := range domains {
				for ip := range a.DetectedIPs {
					// Check if the IP was detected for this application's domain
					if domain == ip { // Simplified for demonstration
						appIPs[appName] = append(appIPs[appName], ip)
					}
				}
			}
		}

		// Write the response as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appIPs)
	})

	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
