package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"sentioxyz/sentio-core/common/flags"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/launcher"
)

func main() {
	var (
		configPath = flag.String("config", "./service/launcher/launcher.yaml", "Path to the launcher configuration file")
	)
	flags.ParseAndInitLogFlag()

	// Load configuration
	cfg, err := launcher.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Infof("Starting service launcher with %d servers", len(cfg.Servers))

	// Create service manager
	serviceManager := launcher.NewServiceManager(cfg)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Start services
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := serviceManager.StartAll(ctx); err != nil {
			log.Errorfe(err, "Failed to start services")
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Infof("Received signal %v, shutting down...", sig)
		cancel()
	case <-ctx.Done():
		log.Info("Context cancelled, shutting down...")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info("Stopping all services...")
	if err := serviceManager.StopAll(shutdownCtx); err != nil {
		log.Errorf("Error during shutdown: %v", err)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	log.Info("Service launcher stopped")
}
