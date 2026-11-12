package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/serjaime99/kvEngine/pkg/server"
	"github.com/serjaime99/kvEngine/pkg/storage"
)

const Banner = `
  _           ______             _            
 | |         |  ____|           (_)           
 | | _____  _| |__   _ __   __ _ _ _ __   ___ 
 | |/ / \ \ / /  __| | '_ \ / _` + "`" + ` | | '_ \ / _ \
 |   <   \ V /| |____| | | | (_| | | | | |  __/
 |_|\_\   \_/ |______|_| |_|\__, |_|_| |_|\___|
                             __/ |            
                            |___/             
`

func main() {
	port := flag.Int("port", 6379, "TCP port to bind the server")
	aofPath := flag.String("aof", "kvengine.aof", "Path to Append-Only File persistence")
	flag.Parse()

	fmt.Println(Banner)
	log.Printf("[kvEngine] Initializing storage engine (64 lock-striped shards)...")

	engine := storage.NewEngine()

	// Replay persistent AOF log if available
	if *aofPath != "" {
		recovered, err := storage.ReplayAOF(*aofPath, engine)
		if err != nil {
			log.Printf("[kvEngine] Warning during AOF recovery: %v", err)
		} else if recovered > 0 {
			log.Printf("[kvEngine] Restored %d commands successfully from '%s'", recovered, *aofPath)
		}
	}

	// Initialize AOF Writer for live persistence
	var aofWriter *storage.AOFWriter
	if *aofPath != "" {
		var err error
		aofWriter, err = storage.NewAOF(*aofPath)
		if err != nil {
			log.Fatalf("[kvEngine] Failed to initialize AOF writer: %v", err)
		}
		engine.SetAOF(aofWriter)
		log.Printf("[kvEngine] AOF persistence active: %s", *aofPath)
	}

	// Start background probabilistic TTL cleaner worker
	stopTTL := make(chan struct{})
	go engine.StartTTLWorker(stopTTL)
	log.Printf("[kvEngine] Active TTL garbage collector running (10Hz sampling)")

	// Initialize Command Dispatcher & TCP Server
	dispatcher := server.NewDispatcher(engine, aofWriter)
	addr := fmt.Sprintf(":%d", *port)
	srv := server.NewServer(addr, dispatcher)

	// Graceful shutdown listener
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[kvEngine] Received shutdown signal. Initiating graceful shutdown...")
		close(stopTTL)
		_ = srv.Stop()
		if aofWriter != nil {
			_ = aofWriter.Sync()
			_ = aofWriter.Close()
			log.Println("[kvEngine] AOF log flushed and safely closed.")
		}
		log.Println("[kvEngine] Shutdown complete. Goodbye.")
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("[kvEngine] Server failure: %v", err)
	}
}
