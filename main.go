// Package main implements Shario, a cross-platform P2P file-sharing application
// with real-time chat capabilities built using libp2p and Fyne.
package main

import (
	"log"
	"shario/internal/app"
)

func main() {
	// Initialize and run the Shario application
	app, err := app.New()
	if err != nil {
		log.Fatal("Failed to initialize application:", err)
	}

	// Start the application
	if err := app.Run(); err != nil {
		log.Fatal("Application error:", err)
	}
}