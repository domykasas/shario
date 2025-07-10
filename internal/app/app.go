// Package app provides the main application structure and initialization
package app

import (
	"context"
	"fmt"
	"log"
	"shario/internal/chat"
	"shario/internal/identity"
	"shario/internal/network"
	"shario/internal/transfer"
	"shario/internal/ui"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

// App represents the main Shario application
type App struct {
	// Core components
	identity *identity.Manager
	network  *network.Manager
	transfer *transfer.Manager
	chat     *chat.Manager
	ui       *ui.Manager

	// Application state
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	fyneApp   fyne.App
	isRunning bool
	mu        sync.RWMutex
}

// New creates a new Shario application instance
func New() (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create Fyne application
	fyneApp := app.New()

	// Initialize identity manager
	identityMgr, err := identity.New()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create identity manager: %w", err)
	}

	// Initialize network manager
	networkMgr, err := network.New(ctx, identityMgr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create network manager: %w", err)
	}

	// Initialize transfer manager
	transferMgr := transfer.New(networkMgr)

	// Initialize chat manager
	chatMgr := chat.New(networkMgr)

	// Initialize UI manager
	uiMgr := ui.New(fyneApp, identityMgr, networkMgr, transferMgr, chatMgr)

	// Set initial nickname in chat manager
	chatMgr.SetNickname(identityMgr.GetNickname())

	return &App{
		identity: identityMgr,
		network:  networkMgr,
		transfer: transferMgr,
		chat:     chatMgr,
		ui:       uiMgr,
		ctx:      ctx,
		cancel:   cancel,
		fyneApp:  fyneApp,
	}, nil
}

// Run starts the Shario application
func (a *App) Run() error {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("application is already running")
	}
	a.isRunning = true
	a.mu.Unlock()

	// Start background services
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

	// Show main window and run GUI
	a.ui.ShowMainWindow()
	a.fyneApp.Run()

	// Cleanup
	a.cancel()
	a.wg.Wait()

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
	a.fyneApp.Quit()
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

// GetPeers returns a list of connected peers for the UI
func (a *App) GetPeers() []*widget.Card {
	peers := a.network.GetPeers()
	cards := make([]*widget.Card, len(peers))

	for i, peer := range peers {
		cards[i] = widget.NewCard(
			peer.Nickname,
			peer.ID,
			widget.NewLabel(fmt.Sprintf("Connected: %s", peer.ConnectedAt.Format("15:04:05"))),
		)
	}

	return cards
}
