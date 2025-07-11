//go:build headless
// +build headless

// Package main implements Shario in headless mode for platforms without GUI support
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"shario/internal/app"
	"syscall"
)

func main() {
	fmt.Println("Shario - P2P File Sharing (Headless Mode)")
	fmt.Println("========================================")
	fmt.Println("Running in headless mode - GUI not available on this platform")
	fmt.Println("P2P networking and file sharing capabilities are available")
	fmt.Println()

	// Initialize the application without GUI
	app, err := app.New()
	if err != nil {
		log.Fatal("Failed to initialize application:", err)
	}

	// Start the application in headless mode
	if err := app.RunHeadless(); err != nil {
		log.Fatal("Application error:", err)
	}

	// Wait for interrupt signal
	fmt.Println("Shario is running in headless mode. Press Ctrl+C to stop.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down...")
}