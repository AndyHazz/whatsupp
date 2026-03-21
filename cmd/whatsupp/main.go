package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/andyhazz/whatsupp/internal/agent"
	"github.com/andyhazz/whatsupp/internal/config"
	"github.com/andyhazz/whatsupp/internal/hub"
	"github.com/andyhazz/whatsupp/internal/version"
)

const defaultConfigPath = "/etc/whatsupp/config.yml"
const defaultAgentConfigPath = "/etc/whatsupp/agent.yml"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: whatsupp <serve|agent>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(version.Version)
	case "serve":
		serve()
	case "agent":
		runAgent()
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

func runAgent() {
	if len(os.Args) >= 3 && os.Args[2] == "init" {
		runAgentInit()
		return
	}

	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	configPath := fs.String("config", defaultAgentConfigPath, "path to agent.yml")
	fs.Parse(os.Args[2:])

	log.Printf("whatsupp agent: loading config from %s", *configPath)
	cfg, err := agent.ParseAgentConfig(*configPath)
	if err != nil {
		log.Fatalf("whatsupp agent: failed to load config: %v", err)
	}

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("whatsupp agent: failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("whatsupp agent: shutting down...")
		cancel()
	}()

	log.Printf("whatsupp agent: starting (hub=%s, host=%s, interval=%s)", cfg.HubURL, cfg.Hostname, cfg.Interval)
	if err := a.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("whatsupp agent: error: %v", err)
	}
}

func runAgentInit() {
	fs := flag.NewFlagSet("agent init", flag.ExitOnError)
	hubURL := fs.String("hub", "", "hub URL (required)")
	key := fs.String("key", "", "agent key (required)")
	hostname := fs.String("hostname", "", "hostname (auto-detected if empty)")
	configPath := fs.String("config", defaultAgentConfigPath, "path to write agent.yml")
	fs.Parse(os.Args[3:])

	if *hubURL == "" || *key == "" {
		fmt.Fprintf(os.Stderr, "Usage: whatsupp agent init --hub URL --key KEY [--hostname NAME] [--config PATH]\n")
		os.Exit(1)
	}

	if err := agent.GenerateConfig(*configPath, *hubURL, *key, *hostname); err != nil {
		log.Fatalf("whatsupp agent init: %v", err)
	}

	log.Printf("whatsupp agent init: config written to %s", *configPath)
}
