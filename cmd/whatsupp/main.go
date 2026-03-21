package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/andyhazz/whatsupp/internal/config"
	"github.com/andyhazz/whatsupp/internal/hub"
)

const defaultConfigPath = "/etc/whatsupp/config.yml"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: whatsupp <serve|agent>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serve()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func serve() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to config.yml")
	fs.Parse(os.Args[2:])

	log.Printf("whatsupp: loading config from %s", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("whatsupp: failed to load config: %v", err)
	}

	h, err := hub.New(cfg, *configPath)
	if err != nil {
		log.Fatalf("whatsupp: failed to create hub: %v", err)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("whatsupp: shutting down...")
		if err := h.Close(); err != nil {
			log.Printf("whatsupp: shutdown error: %v", err)
		}
		os.Exit(0)
	}()

	log.Println("whatsupp: hub starting")
	if err := h.Run(); err != nil {
		log.Fatalf("whatsupp: hub error: %v", err)
	}
}
