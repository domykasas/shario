// Package ui provides the graphical user interface using Fyne
package ui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"shario/internal/chat"
	"shario/internal/identity"
	"shario/internal/network"
	"shario/internal/transfer"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Manager handles the user interface
type Manager struct {
	app      fyne.App
	window   fyne.Window
	identity *identity.Manager
	network  *network.Manager
	transfer *transfer.Manager
	chat     *chat.Manager

	// UI components
	peersList     *widget.List
	transfersList *widget.List
	chatRoomsList *widget.List
	messagesList  *widget.List
	messageEntry  *widget.Entry
	statusLabel   *widget.Label
	nicknameEntry *widget.Entry

	// Data bindings
	peersData     binding.StringList
	transfersData binding.StringList
	roomsData     binding.StringList
	messagesData  binding.StringList

	// Current state
	currentRoom   *chat.Room
	refreshTicker *time.Ticker
}

// Color constants for better UX
var (
	successColor = color.RGBA{R: 46, G: 125, B: 50, A: 255}   // Green for success
	errorColor   = color.RGBA{R: 211, G: 47, B: 47, A: 255}   // Red for errors
	warningColor = color.RGBA{R: 255, G: 152, B: 0, A: 255}   // Orange for warnings
	infoColor    = color.RGBA{R: 33, G: 150, B: 243, A: 255}  // Blue for info
	primaryColor = color.RGBA{R: 103, G: 58, B: 183, A: 255}  // Purple for primary
)

// createColoredLabel creates a label with the specified color
func createColoredLabel(text string, textColor color.Color) *canvas.Text {
	label := canvas.NewText(text, textColor)
	label.TextSize = theme.TextSize()
	return label
}

// createStatusLabel creates a status label with appropriate color
func createStatusLabel(text string, status string) *canvas.Text {
	var textColor color.Color
	switch status {
	case "success", "completed", "connected":
		textColor = successColor
	case "error", "failed", "disconnected":
		textColor = errorColor
	case "warning", "pending", "connecting":
		textColor = warningColor
	case "info", "active":
		textColor = infoColor
	default:
		textColor = theme.ForegroundColor()
	}
	return createColoredLabel(text, textColor)
}

// New creates a new UI manager
func New(fyneApp fyne.App, identityMgr *identity.Manager, networkMgr *network.Manager, transferMgr *transfer.Manager, chatMgr *chat.Manager) *Manager {
	manager := &Manager{
		app:      fyneApp,
		identity: identityMgr,
		network:  networkMgr,
		transfer: transferMgr,
		chat:     chatMgr,
	}

	// Initialize data bindings
	manager.peersData = binding.NewStringList()
	manager.transfersData = binding.NewStringList()
	manager.roomsData = binding.NewStringList()
	manager.messagesData = binding.NewStringList()

	// Set up event handlers
	manager.setupEventHandlers()

	return manager
}

// ShowMainWindow creates and displays the main application window
func (m *Manager) ShowMainWindow() {
	m.window = m.app.NewWindow("Shario - P2P File Sharing")
	m.window.Resize(fyne.NewSize(1200, 800))
	m.window.CenterOnScreen()

	// Create main content
	content := m.createMainContent()
	m.window.SetContent(content)

	// Set up menu
	m.setupMenu()

	// Start refresh ticker
	m.refreshTicker = time.NewTicker(2 * time.Second)
	go m.refreshLoop()

	// Auto-select global room after a short delay
	go func() {
		time.Sleep(1 * time.Second) // Wait for chat manager to initialize
		if globalRoom := m.chat.GetGlobalRoom(); globalRoom != nil {
			m.currentRoom = globalRoom
			m.refreshMessages()
			m.refreshChatRooms()
		}
	}()

	// Show window
	m.window.Show()
}

// createMainContent creates the main content layout
func (m *Manager) createMainContent() *fyne.Container {
	// Create sidebar
	sidebar := m.createSidebar()

	// Create main content area
	mainContent := m.createMainContentArea()

	// Create status bar
	statusBar := m.createStatusBar()

	// Create main layout
	return container.NewBorder(
		nil,         // top
		statusBar,   // bottom
		sidebar,     // left
		nil,         // right
		mainContent, // center
	)
}

// createSidebar creates the sidebar with peers, transfers, and chat rooms
func (m *Manager) createSidebar() *fyne.Container {
	// Create tabs for different sections
	tabs := container.NewAppTabs(
		container.NewTabItem("Peers", m.createPeersTab()),
		container.NewTabItem("Transfers", m.createTransfersTab()),
		container.NewTabItem("Chat", m.createChatTab()),
	)

	tabs.SetTabLocation(container.TabLocationTop)

	// Create user info section
	userInfo := m.createUserInfoSection()

	return container.NewBorder(
		userInfo, // top
		nil,      // bottom
		nil,      // left
		nil,      // right
		tabs,     // center
	)
}

// createUserInfoSection creates the user information section
func (m *Manager) createUserInfoSection() *fyne.Container {
	// Nickname entry
	m.nicknameEntry = widget.NewEntry()
	m.nicknameEntry.SetText(m.identity.GetNickname())
	fmt.Printf("🎭 UI: Created nickname entry with initial text: '%s'\n", m.identity.GetNickname())

	m.nicknameEntry.OnSubmitted = func(text string) {
		fmt.Printf("🎭 UI: Nickname change requested: '%s'\n", text)
		if err := m.identity.SetNickname(text); err != nil {
			m.showError("Failed to set nickname", err)
		} else {
			fmt.Printf("🎭 UI: Calling chat.SetNickname with '%s'\n", text)
			m.chat.SetNickname(text)
			// Update the UI field to reflect the change
			m.nicknameEntry.SetText(text)
			fmt.Printf("🎭 UI: Updated nickname entry field to '%s'\n", text)
		}
	}

	// Also add OnChanged callback to see if the field is being edited
	m.nicknameEntry.OnChanged = func(text string) {
		fmt.Printf("🎭 UI: Nickname entry changed to: '%s'\n", text)
	}

	// Add update button as backup method for nickname changes
	updateNicknameBtn := widget.NewButton("Update", func() {
		currentText := m.nicknameEntry.Text
		fmt.Printf("🎭 UI: Update button clicked with text: '%s'\n", currentText)

		if strings.TrimSpace(currentText) != "" {
			if err := m.identity.SetNickname(currentText); err != nil {
				m.showError("Failed to set nickname", err)
			} else {
				fmt.Printf("🎭 UI: Update button - calling chat.SetNickname with '%s'\n", currentText)
				m.chat.SetNickname(currentText)
				m.nicknameEntry.SetText(currentText)
				fmt.Printf("🎭 UI: Update button - updated nickname entry field to '%s'\n", currentText)
			}
		}
	})

	// Peer ID label
	peerIDLabel := widget.NewLabel(fmt.Sprintf("ID: %s", m.identity.GetPeerID().String()))

	return container.NewVBox(
		widget.NewCard("🎭 Your Identity", "",
			container.NewVBox(
				widget.NewLabel("Nickname:"),
				container.NewBorder(
					nil, nil, nil, updateNicknameBtn, // button on the right
					m.nicknameEntry, // entry field takes the main space
				),
				peerIDLabel,
			),
		),
		widget.NewSeparator(),
	)
}

// createPeersTab creates the peers tab
func (m *Manager) createPeersTab() *fyne.Container {
	// Create peers list
	m.peersList = widget.NewListWithData(
		m.peersData,
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				container.NewHBox(
					widget.NewButton("Chat", nil),
					widget.NewButton("Send File", nil),
				),
				container.NewVBox(
					widget.NewLabel("Peer Name"),
					widget.NewLabel("Peer ID"),
				),
			)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			text, _ := item.(binding.String).Get()
			parts := strings.Split(text, "|")
			if len(parts) >= 2 {
				cont := obj.(*fyne.Container)
				vbox := cont.Objects[0].(*fyne.Container)
				hbox := cont.Objects[1].(*fyne.Container)

				nameLabel := vbox.Objects[0].(*widget.Label)
				idLabel := vbox.Objects[1].(*widget.Label)
				chatBtn := hbox.Objects[0].(*widget.Button)
				sendFileBtn := hbox.Objects[1].(*widget.Button)

				nameLabel.SetText(parts[0])
				idLabel.SetText(parts[1])

				// Set button callbacks
				chatBtn.OnTapped = func() {
					m.startChatWithPeer(parts[1])
				}
				sendFileBtn.OnTapped = func() {
					m.sendFileToProj(parts[1])
				}
			}
		},
	)

	// Add refresh button and status info
	refreshBtn := widget.NewButton("Refresh Peers", func() {
		m.refreshPeers()
	})

	// Add manual connection button
	connectBtn := widget.NewButton("Connect to Peer", func() {
		m.showConnectToPeerDialog()
	})

	// Add peer count and connection info
	peerCountLabel := widget.NewLabel("Peers: 0")
	hostInfoLabel := widget.NewLabel(fmt.Sprintf("Host: %s", m.identity.GetPeerID().String()))

	// Update peer count label periodically
	go func() {
		for range time.Tick(2 * time.Second) {
			count := m.network.GetPeerCount()
			peerCountLabel.SetText(fmt.Sprintf("Peers: %d", count))
		}
	}()

	// Create colored header
	peersHeaderText := createColoredLabel("👥 Connected Peers", primaryColor)
	peersHeaderText.TextStyle = fyne.TextStyle{Bold: true}
	
	return container.NewVBox(
		peersHeaderText,
		peerCountLabel,
		hostInfoLabel,
		widget.NewSeparator(),
		m.peersList,
		widget.NewSeparator(),
		container.NewHBox(refreshBtn, connectBtn),
	)
}

// createTransfersTab creates the transfers tab
func (m *Manager) createTransfersTab() *fyne.Container {
	// Create transfers list
	m.transfersList = widget.NewListWithData(
		m.transfersData,
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				container.NewHBox(
					widget.NewButton("Cancel", nil),
					widget.NewButton("Open", nil),
				),
				container.NewVBox(
					widget.NewLabel("Filename"),
					widget.NewProgressBar(),
					widget.NewLabel("Status"),
				),
			)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			text, _ := item.(binding.String).Get()
			parts := strings.Split(text, "|")
			if len(parts) >= 4 {
				cont := obj.(*fyne.Container)
				vbox := cont.Objects[0].(*fyne.Container)
				hbox := cont.Objects[1].(*fyne.Container)

				nameLabel := vbox.Objects[0].(*widget.Label)
				progressBar := vbox.Objects[1].(*widget.ProgressBar)
				statusLabel := vbox.Objects[2].(*widget.Label)
				cancelBtn := hbox.Objects[0].(*widget.Button)
				openBtn := hbox.Objects[1].(*widget.Button)

				nameLabel.SetText(parts[0])
				
				// Set colored status text
				status := parts[1]
				statusLabel.SetText(status)
				switch status {
				case "completed":
					statusLabel.TextStyle = fyne.TextStyle{Bold: true}
					// Note: Fyne doesn't support setting label colors directly, 
					// but we can use importance styling
				case "failed", "cancelled":
					statusLabel.TextStyle = fyne.TextStyle{Bold: true}
				case "active":
					statusLabel.TextStyle = fyne.TextStyle{Italic: true}
				case "pending":
					statusLabel.TextStyle = fyne.TextStyle{}
				}

				// Parse progress
				var progress float64
				fmt.Sscanf(parts[2], "%f", &progress)
				progressBar.SetValue(progress / 100.0)

				// Set button callbacks
				transferID := parts[3]
				cancelBtn.OnTapped = func() {
					fmt.Printf("🗂️ UI: Cancel button clicked for transfer %s\n", transferID)
					if err := m.transfer.CancelTransfer(transferID); err != nil {
						m.showError("Failed to cancel transfer", err)
					}
				}
				openBtn.OnTapped = func() {
					fmt.Printf("🗂️ UI: Open button clicked for transfer %s\n", transferID)
					m.openTransferLocation(transferID)
				}
			}
		},
	)

	// Create colored header
	headerText := createColoredLabel("📁 File Transfers", primaryColor)
	headerText.TextStyle = fyne.TextStyle{Bold: true}
	
	return container.NewVBox(
		headerText,
		widget.NewSeparator(),
		m.transfersList,
	)
}

// createChatTab creates the chat tab
func (m *Manager) createChatTab() *fyne.Container {
	// Create chat rooms list
	m.chatRoomsList = widget.NewListWithData(
		m.roomsData,
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				widget.NewLabel("0"), // unread count
				container.NewVBox(
					widget.NewLabel("Room Name"),
					widget.NewLabel("Last Message"),
				),
			)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			text, _ := item.(binding.String).Get()
			parts := strings.Split(text, "|")
			if len(parts) >= 3 {
				cont := obj.(*fyne.Container)
				vbox := cont.Objects[0].(*fyne.Container)
				unreadLabel := cont.Objects[1].(*widget.Label)

				nameLabel := vbox.Objects[0].(*widget.Label)
				lastMsgLabel := vbox.Objects[1].(*widget.Label)

				nameLabel.SetText(parts[0])
				lastMsgLabel.SetText(parts[1])
				unreadLabel.SetText(parts[2])
			}
		},
	)

	// Set selection callback
	m.chatRoomsList.OnSelected = func(id widget.ListItemID) {
		rooms := m.chat.GetRooms()
		if id < len(rooms) {
			m.currentRoom = rooms[id]
			m.refreshMessages()
			m.chat.MarkRoomAsRead(m.currentRoom.ID)
		}
	}

	// Add global chat info
	globalChatInfo := widget.NewLabel("Global chat connects all Shario users automatically")
	globalChatInfo.Wrapping = fyne.TextWrapWord

	// Create colored header
	chatHeaderText := createColoredLabel("💬 Chat Rooms", primaryColor)
	chatHeaderText.TextStyle = fyne.TextStyle{Bold: true}
	
	return container.NewVBox(
		chatHeaderText,
		globalChatInfo,
		widget.NewSeparator(),
		m.chatRoomsList,
	)
}

// createMainContentArea creates the main content area
func (m *Manager) createMainContentArea() *fyne.Container {
	// Create message list
	m.messagesList = widget.NewListWithData(
		m.messagesData,
		func() fyne.CanvasObject {
			// Single line format: [Time] Sender: Message
			return widget.NewLabel("Message placeholder")
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			text, _ := item.(binding.String).Get()
			parts := strings.Split(text, "|")
			if len(parts) >= 3 {
				label := obj.(*widget.Label)
				// Format: [HH:MM:SS] Sender: Message
				compactMessage := fmt.Sprintf("[%s] %s: %s", parts[2], parts[0], parts[1])
				label.SetText(compactMessage)
			}
		},
	)

	// Create message entry
	m.messageEntry = widget.NewEntry()
	m.messageEntry.SetPlaceHolder("Type a message to global chat...")
	m.messageEntry.MultiLine = true
	m.messageEntry.OnSubmitted = func(text string) {
		if m.currentRoom == nil {
			// Auto-select global room if available
			if globalRoom := m.chat.GetGlobalRoom(); globalRoom != nil {
				m.currentRoom = globalRoom
			} else {
				m.showError("Global chat not ready", fmt.Errorf("global chat is initializing, please wait a moment"))
				return
			}
		}
		if strings.TrimSpace(text) == "" {
			return
		}
		m.chat.SendMessage(m.currentRoom.ID, text)
		m.messageEntry.SetText("")
	}

	// Create send button
	sendBtn := widget.NewButton("Send", func() {
		if m.currentRoom == nil {
			// Auto-select global room if available
			if globalRoom := m.chat.GetGlobalRoom(); globalRoom != nil {
				m.currentRoom = globalRoom
			} else {
				m.showError("Global chat not ready", fmt.Errorf("global chat is initializing, please wait a moment"))
				return
			}
		}
		if strings.TrimSpace(m.messageEntry.Text) == "" {
			return
		}
		m.chat.SendMessage(m.currentRoom.ID, m.messageEntry.Text)
		m.messageEntry.SetText("")
	})

	// Create message input area
	messageInput := container.NewBorder(
		nil, nil, nil, sendBtn,
		m.messageEntry,
	)

	return container.NewBorder(
		nil,            // top
		messageInput,   // bottom
		nil,            // left
		nil,            // right
		m.messagesList, // center
	)
}

// createStatusBar creates the status bar
func (m *Manager) createStatusBar() *fyne.Container {
	m.statusLabel = widget.NewLabel("Ready")
	m.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	// Create colored status indicators
	statusText := createStatusLabel("Ready", "success")
	peersText := createColoredLabel("Peers: 0", infoColor)
	transfersText := createColoredLabel("Transfers: 0", infoColor)
	
	return container.NewHBox(
		statusText,
		widget.NewSeparator(),
		peersText,
		widget.NewSeparator(),
		transfersText,
	)
}

// setupMenu sets up the application menu
func (m *Manager) setupMenu() {
	// File menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Send File", func() {
			m.showFileSendDialog()
		}),
		fyne.NewMenuItem("Download Folder", func() {
			m.showDownloadFolderDialog()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", func() {
			m.app.Quit()
		}),
	)

	// Settings menu
	settingsMenu := fyne.NewMenu("Settings",
		fyne.NewMenuItem("Change Nickname", func() {
			m.showNicknameDialog()
		}),
		fyne.NewMenuItem("Export Identity", func() {
			m.showExportIdentityDialog()
		}),
		fyne.NewMenuItem("Import Identity", func() {
			m.showImportIdentityDialog()
		}),
	)

	// Help menu
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			m.showAboutDialog()
		}),
	)

	mainMenu := fyne.NewMainMenu(fileMenu, settingsMenu, helpMenu)
	m.window.SetMainMenu(mainMenu)
}

// setupEventHandlers sets up event handlers for backend components
func (m *Manager) setupEventHandlers() {
	// Chat event handlers
	m.chat.SetMessageHandler(func(msg *chat.Message) {
		m.refreshMessages()
		m.refreshChatRooms()
		// Refresh peers list in case nicknames changed
		m.refreshPeers()
	})

	m.chat.SetRoomUpdateHandler(func(room *chat.Room) {
		m.refreshChatRooms()
		// Refresh peers list in case nicknames changed
		m.refreshPeers()
	})

	// Transfer event handlers
	m.transfer.SetTransferUpdateHandler(func(transfer *transfer.Transfer) {
		m.refreshTransfers()
	})

	m.transfer.SetTransferOfferHandler(func(transfer *transfer.Transfer) bool {
		return m.showTransferOfferDialog(transfer)
	})
}

// refreshLoop periodically refreshes the UI
func (m *Manager) refreshLoop() {
	for range m.refreshTicker.C {
		m.refreshPeers()
		m.refreshTransfers()
		m.refreshChatRooms()
		m.updateStatusCounts()
	}
}

// refreshPeers refreshes the peers list
func (m *Manager) refreshPeers() {
	peers := m.network.GetPeers()
	var peerStrings []string

	for _, peer := range peers {
		peerString := fmt.Sprintf("%s|%s", peer.Nickname, peer.ID)
		peerStrings = append(peerStrings, peerString)
	}

	m.peersData.Set(peerStrings)
}

// refreshTransfers refreshes the transfers list
func (m *Manager) refreshTransfers() {
	transfers := m.transfer.GetTransfers()
	var transferStrings []string

	for _, transfer := range transfers {
		// Add emoji based on status
		var statusEmoji string
		switch transfer.Status {
		case "completed":
			statusEmoji = "✅"
		case "failed":
			statusEmoji = "❌"
		case "cancelled":
			statusEmoji = "🚫"
		case "active":
			statusEmoji = "🔄"
		case "pending":
			statusEmoji = "⏳"
		default:
			statusEmoji = "📄"
		}
		
		transferString := fmt.Sprintf("%s|%s %s|%.1f|%s",
			transfer.Filename, statusEmoji, transfer.Status, transfer.Progress, transfer.ID)
		transferStrings = append(transferStrings, transferString)
	}

	m.transfersData.Set(transferStrings)
}

// refreshChatRooms refreshes the chat rooms list
func (m *Manager) refreshChatRooms() {
	rooms := m.chat.GetRooms()
	var roomStrings []string

	for _, room := range rooms {
		lastMsg := "No messages"
		if room.LastMessage != nil {
			lastMsg = room.LastMessage.Content
			if len(lastMsg) > 30 {
				lastMsg = lastMsg[:30] + "..."
			}
		}

		roomString := fmt.Sprintf("%s|%s|%d", room.Name, lastMsg, room.UnreadCount)
		roomStrings = append(roomStrings, roomString)
	}

	m.roomsData.Set(roomStrings)
}

// refreshMessages refreshes the messages list for current room
func (m *Manager) refreshMessages() {
	if m.currentRoom == nil {
		m.messagesData.Set([]string{})
		return
	}

	var messageStrings []string
	for _, msg := range m.currentRoom.Messages {
		timeStr := msg.Timestamp.Format("15:04:05")
		msgString := fmt.Sprintf("%s|%s|%s", msg.Sender, msg.Content, timeStr)
		messageStrings = append(messageStrings, msgString)
	}

	m.messagesData.Set(messageStrings)
}

// updateStatusCounts updates the status bar with peer and transfer counts
func (m *Manager) updateStatusCounts() {
	peerCount := m.network.GetPeerCount()
	transferCount := m.transfer.GetActiveTransfers()

	status := fmt.Sprintf("Peers: %d | Transfers: %d", peerCount, transferCount)
	m.statusLabel.SetText(status)
}

// startChatWithPeer starts a chat with a peer
func (m *Manager) startChatWithPeer(peerIDStr string) {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		m.showError("Invalid peer ID", err)
		return
	}

	// Find peer info
	peers := m.network.GetPeers()
	var selectedPeer *network.Peer
	for _, peer := range peers {
		if peer.PeerID == peerID {
			selectedPeer = peer
			break
		}
	}

	if selectedPeer == nil {
		m.showError("Peer not found", fmt.Errorf("peer %s not found", peerIDStr))
		return
	}

	// Create or get existing room
	room := m.chat.CreateDirectRoom(peerID, selectedPeer.Nickname)
	m.currentRoom = room
	m.refreshMessages()
	m.refreshChatRooms()
}

// connectToPeerManually attempts to connect to a peer using their multiaddress
func (m *Manager) connectToPeerManually(addrStr string) {
	// Parse the multiaddress
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		m.showError("Invalid address", fmt.Errorf("failed to parse multiaddress: %w", err))
		return
	}

	// Extract peer info
	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		m.showError("Invalid peer address", fmt.Errorf("failed to extract peer info: %w", err))
		return
	}

	// Attempt connection
	go func() {
		if err := m.network.GetHost().Connect(context.Background(), *peerInfo); err != nil {
			m.showError("Connection failed", fmt.Errorf("failed to connect to peer: %w", err))
		} else {
			// Connection successful - peer should appear in the list automatically
			dialog.ShowInformation("Success", "Successfully connected to peer!", m.window)
		}
	}()
}

// sendFileToProj sends a file to a peer
func (m *Manager) sendFileToProj(peerIDStr string) {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		m.showError("Invalid peer ID", err)
		return
	}

	// Show file picker
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			m.showError("Failed to open file", err)
			return
		}
		if reader != nil {
			defer reader.Close()

			// Get file path
			filePath := reader.URI().Path()

			// Send file
			if _, err := m.transfer.SendFile(peerID, filePath); err != nil {
				m.showError("Failed to send file", err)
			}
		}
	}, m.window)

	fileDialog.Show()
}

// showError displays an error dialog
func (m *Manager) showError(title string, err error) {
	dialog.ShowError(err, m.window)
}

// showFileSendDialog shows the file send dialog
func (m *Manager) showFileSendDialog() {
	// Implementation for file send dialog
	// TODO: Implement file send dialog
}

// showDownloadFolderDialog shows the download folder dialog
func (m *Manager) showDownloadFolderDialog() {
	// Implementation for download folder dialog
	// TODO: Implement download folder dialog
}

// showNicknameDialog shows the nickname change dialog
func (m *Manager) showNicknameDialog() {
	fmt.Printf("🎭 UI: Opening nickname dialog\n")
	entry := widget.NewEntry()
	entry.SetText(m.identity.GetNickname())

	dialog.ShowForm("Change Nickname", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Nickname", entry),
	}, func(accepted bool) {
		fmt.Printf("🎭 UI Dialog: Dialog callback called, accepted: %t\n", accepted)
		if accepted {
			fmt.Printf("🎭 UI Dialog: Nickname change accepted: '%s'\n", entry.Text)
			if err := m.identity.SetNickname(entry.Text); err != nil {
				m.showError("Failed to set nickname", err)
			} else {
				fmt.Printf("🎭 UI Dialog: Calling chat.SetNickname with '%s'\n", entry.Text)
				m.chat.SetNickname(entry.Text)
				m.nicknameEntry.SetText(entry.Text)
				fmt.Printf("🎭 UI Dialog: Updated main nickname entry to '%s'\n", entry.Text)
			}
		}
	}, m.window)
}

// showExportIdentityDialog shows the identity export dialog
func (m *Manager) showExportIdentityDialog() {
	// Implementation for identity export dialog
	// TODO: Implement identity export dialog
}

// showImportIdentityDialog shows the identity import dialog
func (m *Manager) showImportIdentityDialog() {
	// Implementation for identity import dialog
	// TODO: Implement identity import dialog
}

// showConnectToPeerDialog shows manual peer connection dialog
func (m *Manager) showConnectToPeerDialog() {
	peerAddrEntry := widget.NewEntry()
	peerAddrEntry.SetPlaceHolder("/ip4/192.168.1.100/tcp/12345/p2p/QmYWdN8PKoFFNFBNCeM6VsDrzzs1QQacLsmWAx3WLHTtGR")
	peerAddrEntry.MultiLine = true

	helpText := widget.NewLabel("Enter a peer's multiaddress. You can get this from another Shario instance's console output.")

	dialog.ShowForm("Connect to Peer", "Connect", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Peer Address", peerAddrEntry),
		widget.NewFormItem("Help", helpText),
	}, func(accepted bool) {
		if accepted && strings.TrimSpace(peerAddrEntry.Text) != "" {
			m.connectToPeerManually(strings.TrimSpace(peerAddrEntry.Text))
		}
	}, m.window)
}

// showAboutDialog shows the about dialog
func (m *Manager) showAboutDialog() {
	dialog.ShowInformation("About Shario",
		"Shario v1.0.0\n\nA cross-platform P2P file sharing application\nwith real-time chat capabilities.\n\nBuilt with Go, libp2p, and Fyne.",
		m.window)
}

// openTransferLocation opens the file or folder location for a transfer
func (m *Manager) openTransferLocation(transferID string) {
	// Get transfer info
	transfers := m.transfer.GetTransfers()
	var targetTransfer *transfer.Transfer
	for _, t := range transfers {
		if t.ID == transferID {
			targetTransfer = t
			break
		}
	}

	if targetTransfer == nil {
		m.showError("Transfer not found", fmt.Errorf("transfer %s not found", transferID))
		return
	}

	// Check if transfer is completed and file exists
	if targetTransfer.Status == "completed" && targetTransfer.FilePath != "" {
		// Try to open the file
		if err := m.openFileInSystem(targetTransfer.FilePath); err != nil {
			// If opening file fails, try opening the folder
			folderPath := filepath.Dir(targetTransfer.FilePath)
			if err2 := m.openFileInSystem(folderPath); err2 != nil {
				m.showError("Failed to open file location", fmt.Errorf("cannot open file or folder: %v", err2))
			}
		}
	} else {
		// Transfer not completed or no file path, open download folder
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.Getenv("HOME") // fallback
		}
		downloadDir := filepath.Join(homeDir, "Downloads", "Shario")
		if err := m.openFileInSystem(downloadDir); err != nil {
			m.showError("Failed to open download folder", err)
		}
	}
}

// openFileInSystem opens a file or folder using the system's default application
func (m *Manager) openFileInSystem(path string) error {
	fmt.Printf("🗂️ UI: Opening system path: %s\n", path)

	// Cross-platform file opening
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = "open"
		args = []string{path}
	case "linux":
		cmd = "xdg-open"
		args = []string{path}
	case "windows":
		cmd = "explorer"
		args = []string{path}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	exec := exec.Command(cmd, args...)
	return exec.Start()
}

// showTransferOfferDialog shows a transfer offer dialog
func (m *Manager) showTransferOfferDialog(transfer *transfer.Transfer) bool {
	fmt.Printf("🎯 UI: Showing transfer offer dialog for file: %s\n", transfer.Filename)

	content := fmt.Sprintf("Peer %s wants to send you a file:\n\nFilename: %s\nSize: %d bytes\n\nDo you want to accept this transfer?",
		transfer.PeerNickname, transfer.Filename, transfer.Size)

	// Use a channel to wait for user response
	responseChan := make(chan bool, 1)

	dialog.ShowConfirm("File Transfer Request", content, func(accepted bool) {
		fmt.Printf("🎯 UI: User clicked on transfer dialog, accepted: %t\n", accepted)
		responseChan <- accepted
	}, m.window)

	// Wait for user response
	accepted := <-responseChan
	fmt.Printf("🎯 UI: Transfer dialog result: %t\n", accepted)
	return accepted
}
