//go:build headless
// +build headless

// Package app provides the main application structure for headless mode
package app

import (
	"context"
	"fmt"
	"log"
	"shario/internal/chat"
	"shario/internal/identity"
	"shario/internal/network"
	"shario/internal/transfer"
	"sync"
)

// App represents the main Shario application in headless mode
type App struct {
	// Core components (no UI manager)
	identity *identity.Manager
	network  *network.Manager
	transfer *transfer.Manager
	chat     *chat.Manager

	// Application state
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex
}

// New creates a new Shario application instance in headless mode
func New() (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize identity manager
	identityMgr, err := identity.New()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create identity manager: %w", err)
	}

	// Initialize network manager
	networkMgr, err := network.New(identityMgr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}

	// Initialize transfer manager
	transferMgr := transfer.New(networkMgr)

	// Initialize chat manager
	chatMgr := chat.New(networkMgr, identityMgr)

	// Create application instance
	app := &App{
		identity: identityMgr,
		network:  networkMgr,
		transfer: transferMgr,
		chat:     chatMgr,
		ctx:      ctx,
		cancel:   cancel,
	}

	return app, nil
}

// Run starts the application in headless mode (not used in headless builds)
func (a *App) Run() error {
	return fmt.Errorf("Run() not available in headless mode, use RunHeadless() instead")
}

// RunHeadless starts the application without GUI
func (a *App) RunHeadless() error {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	a.isRunning = true
	a.mu.Unlock()

	log.Println("Starting Shario in headless mode...")

	// Start all managers except UI
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.network.Start(); err != nil {
			log.Printf("Network manager error: %v", err)
		}
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.chat.Start(); err != nil {
			log.Printf("Chat manager error: %v", err)
		}
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		if err := a.transfer.Start(); err != nil {
			log.Printf("Transfer manager error: %v", err)
		}
	}()

	log.Printf("Shario headless mode started successfully")
	log.Printf("Identity: %s", a.identity.GetNickname())
	log.Printf("Peer ID: %s", a.identity.GetPeerID())
	log.Printf("Listening for peers...")

	return nil
}

// Shutdown gracefully stops the application
func (a *App) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return
	}

	a.isRunning = false
	a.cancel()
	// No GUI to quit in headless mode
}

// GetStatus returns the current application status
func (a *App) GetStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := map[string]interface{}{
		"running":    a.isRunning,
		"peers":      a.network.GetPeerCount(),
		"identity":   a.identity.GetNickname(),
		"transfers":  a.transfer.GetActiveTransfers(),
		"chat_rooms": a.chat.GetActiveRooms(),
	}

	return status
}