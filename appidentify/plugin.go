package appidentify

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"sync"

	"github.com/coredns/caddy"
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
	log.Printf("[ServeDNS] Received DNS query for: %s", r.Question[0].Name)

	// Check if the query matches any application domain
	for appName, domains := range a.AppDirectory {
		log.Printf("[ServeDNS] Checking application: %s", appName)
		for _, domain := range domains {
			log.Printf("[ServeDNS] Comparing query %s with domain %s", r.Question[0].Name, domain)
			if r.Question[0].Name == domain {
				log.Printf("[ServeDNS] Match found for application: %s", appName)

				// Extract IP addresses from the DNS response
				for _, answer := range r.Answer {
					log.Printf("[ServeDNS] Processing DNS answer: %v", answer)
					if aRecord, ok := answer.(*dns.A); ok {
						ip := aRecord.A.String()

						// Thread-safe addition to DetectedIPs
						a.mu.Lock()
						a.DetectedIPs[ip] = struct{}{}
						a.mu.Unlock()

						log.Printf("[ServeDNS] Added IP %s for application %s", ip, appName)

						// Update the Linux kernel IP set
						if err := addToIPSet(ip); err != nil {
							log.Printf("[ServeDNS] Failed to add IP %s to IP set: %v", ip, err)
						} else {
							log.Printf("[ServeDNS] Successfully added IP %s to IP set", ip)
						}
					}
				}
			}
		}
	}

	log.Printf("[ServeDNS] Passing query to the next plugin in the chain")
	return plugin.NextOrFailure(a.Name(), a.Next, ctx, w, r)
}

func (a *AppIdentifyPlugin) Name() string {
	return "appidentify"
}

func Setup(appDirectoryPath string) (*AppIdentifyPlugin, error) {
	log.Printf("[Setup] Loading application directory from: %s", appDirectoryPath)
	// Load application directory from JSON file
	appDirectory, err := loadAppDirectory(appDirectoryPath)
	if err != nil {
		return nil, err
	}

	a := &AppIdentifyPlugin{
		AppDirectory: appDirectory,
		DetectedIPs:  make(map[string]struct{}),
	}

	// Start the HTTP server in a separate goroutine
	go startHTTPServer(a)

	return a, nil
}

func loadAppDirectory(filePath string) (map[string][]string, error) {
	log.Printf("[loadAppDirectory] Reading application directory from file: %s", filePath)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("[loadAppDirectory] Failed to read or parse file: %v", err)
		return nil, err
	}

	var appDirectory map[string][]string
	if err := json.Unmarshal(data, &appDirectory); err != nil {
		log.Printf("[loadAppDirectory] Failed to read or parse file: %v", err)
		return nil, err
	}
	log.Printf("[loadAppDirectory] Successfully loaded application directory")
	return appDirectory, nil
}

func addToIPSet(ip string) error {
	log.Printf("[addToIPSet] Adding IP %s to IP set", ip)
	cmd := exec.Command("ipset", "add", "detected_ips", ip, "-exist") // Use -exist to avoid duplicate errors
	if err := cmd.Run(); err != nil {
		log.Printf("[addToIPSet] Error adding IP %s to IP set: %v", ip, err)
		return err
	}
	log.Printf("[addToIPSet] Successfully added IP %s to IP set", ip)
	return nil
}

func startHTTPServer(a *AppIdentifyPlugin) {
	log.Println("[startHTTPServer] Starting HTTP server on :8080")
	http.HandleFunc("/detected", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[startHTTPServer] Received request for /detected")
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
		log.Println("[startHTTPServer] Responded with detected applications and IPs")
	})

	log.Println("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// setup is called by CoreDNS to initialize the plugin.
func setup(c *caddy.Controller) error {
	log.Println("[setup] Called by CoreDNS to initialize the AppIdentify plugin") // Added log

	appDirectoryPath := "appidentify/applications.json" // Default path to the JSON file

	// Initialize the plugin
	_, err := Setup(appDirectoryPath)
	if err != nil {
		return err
	}

	// Register the plugin in the CoreDNS plugin chain
	c.OnStartup(func() error {
		log.Println("AppIdentify plugin setup complete")
		return nil
	})

	c.OnShutdown(func() error {
		log.Println("AppIdentify plugin shutting down")
		return nil
	})

	return nil
}
