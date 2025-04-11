package proxy

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/rodrwan/shareiscare/config"
)

func Start(cfg *config.Config, Domain, TunnelURL, ZoneID, ApiToken, TunnelName string, embeddedBinaries embed.FS) {
	log.Println("⌛ Checking if the server is listening on localhost:" + strconv.Itoa(cfg.Port) + "...")

	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			log.Fatal("❌ Server did not respond on port " + strconv.Itoa(cfg.Port) + " after 5s. Aborting.")
		case <-ticker.C:
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d", cfg.Port))
			if err == nil {
				resp.Body.Close()
				log.Println("✅ Server is ready. Starting tunnel...")
				goto RUN_TUNNEL
			}
		}
	}

RUN_TUNNEL:
	hostname := cfg.Hostname

	if hostname == "" {
		newDnsRecord, err := CreateDNSRecord(Domain, TunnelURL, ZoneID, ApiToken)
		if err != nil {
			log.Panicf("❌ Could not create DNS record: %v", err)
		}
		hostname = newDnsRecord
		config.SetHostname(hostname)
	}

	log.Println("⚙️ provisioning hostname", hostname)

	// Check if we're running in a test environment
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		log.Println("⚠️ Running in GitHub Actions environment, skipping cloudflared setup")
		return
	}

	binPath, err := ExtractCloudflaredBinary(embeddedBinaries)
	if err != nil {
		log.Panicf("❌ Error extracting cloudflared: %v", err)
	}

	log.Println("🚀 Launching cloudflared...")
	tmpFile, err := RunCloudflared(binPath, hostname, cfg.Port, TunnelName)
	if err != nil {
		log.Panicf("❌ Error running cloudflared: %v", err)
	}
	defer os.Remove(tmpFile) // Clean up temporary file when done

	// Run cloudflared with temporary configuration file
	cmd := exec.Command(binPath,
		"tunnel",
		"--config", tmpFile,
		"run",
		TunnelName,
	)

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Panicf("❌ Error starting cloudflared: %v", err)
	}

	// Wait a moment to see if the tunnel establishes
	time.Sleep(5 * time.Second)

	ticker = time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("🔍 Checking if the tunnel is established...")
			resp, err := http.Get(fmt.Sprintf("https://%s", hostname))
			if err == nil {
				resp.Body.Close()
				log.Println("✅ Tunnel is established. Starting server...")
				goto RUN_SERVER
			}
		}
	}

RUN_SERVER:
	// Check if the process is still running
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		log.Panicf("❌ cloudflared closed unexpectedly")
	}

	log.Println("🌐 You can access the server at https://" + hostname)
	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		log.Panicf("❌ Error waiting for command to finish: %v", err)
	}

	fmt.Println("👋 Goodbye!")
	os.Exit(0)
}
