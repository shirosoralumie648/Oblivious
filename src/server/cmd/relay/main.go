package main

import (
	"log"
	"os"
)

func main() {
	mode := os.Getenv("OBLIVIOUS_DEPLOYMENT_MODE")
	if mode == "" {
		mode = "monolith"
	}

	switch mode {
	case "monolith":
		log.Println("Starting in monolith mode (all services in one process)")
		// Current cmd/server/main.go behavior
		log.Fatal("Monolith mode: use cmd/server/main.go directly")
	case "microservices":
		log.Println("Starting in microservices mode (relay service only)")
		log.Fatal("Microservices mode: relay service entry point not yet implemented (Stage C3)")
	default:
		log.Fatalf("Unknown OBLIVIOUS_DEPLOYMENT_MODE: %s (valid: monolith, microservices)", mode)
	}
}
