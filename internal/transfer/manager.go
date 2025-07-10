// Package transfer handles P2P file transfers with progress tracking
package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"shario/internal/network"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Transfer represents a file transfer
type Transfer struct {
	ID            string          `json:"id"`
	Filename      string          `json:"filename"`
	Size          int64           `json:"size"`
	Transferred   int64           `json:"transferred"`
	Speed         int64           `json:"speed"`         // bytes per second
	Progress      float64         `json:"progress"`      // 0-100
	Status        TransferStatus  `json:"status"`
	Direction     TransferDirection `json:"direction"`
	PeerID        peer.ID         `json:"peer_id"`
	PeerNickname  string          `json:"peer_nickname"`
	FilePath      string          `json:"file_path"`
	Checksum      string          `json:"checksum"`
	StartTime     time.Time       `json:"start_time"`
	EndTime       *time.Time      `json:"end_time,omitempty"`
	Error         string          `json:"error,omitempty"`
	
	// Internal fields
	file          *os.File
	cancel        context.CancelFunc
	lastUpdate    time.Time
}

// TransferStatus represents the status of a transfer
type TransferStatus string

const (
	StatusPending    TransferStatus = "pending"
	StatusActive     TransferStatus = "active"
	StatusCompleted  TransferStatus = "completed"
	StatusFailed     TransferStatus = "failed"
	StatusCancelled  TransferStatus = "cancelled"
	StatusPaused     TransferStatus = "paused"
)

// TransferDirection represents the direction of a transfer
type TransferDirection string

const (
	DirectionSend    TransferDirection = "send"
	DirectionReceive TransferDirection = "receive"
)

// TransferMessage represents a transfer protocol message
type TransferMessage struct {
	Type     string                 `json:"type"`
	Data     map[string]interface{} `json:"data"`
}

// Message types
const (
	MsgTypeOffer    = "offer"
	MsgTypeAccept   = "accept"
	MsgTypeReject   = "reject"
	MsgTypeData     = "data"
	MsgTypeComplete = "complete"
	MsgTypeCancel   = "cancel"
	MsgTypeProgress = "progress"
)

// Manager handles file transfers
type Manager struct {
	network     *network.Manager
	transfers   map[string]*Transfer
	mutex       sync.RWMutex
	downloadDir string
	maxFileSize int64
	
	// Event handlers
	onTransferUpdate func(*Transfer)
	onTransferOffer  func(*Transfer) bool // returns true to accept
}

// New creates a new transfer manager
func New(networkMgr *network.Manager) *Manager {
	homeDir, _ := os.UserHomeDir()
	downloadDir := filepath.Join(homeDir, "Downloads", "Shario")
	
	// Create download directory if it doesn't exist
	os.MkdirAll(downloadDir, 0755)
	
	mgr := &Manager{
		network:     networkMgr,
		transfers:   make(map[string]*Transfer),
		downloadDir: downloadDir,
		maxFileSize: 1024 * 1024 * 1024, // 1GB default limit
	}
	
	// Register as network event handler
	networkMgr.AddEventHandler("transfer", mgr)
	
	return mgr
}

// Start initializes the transfer manager
func (m *Manager) Start() error {
	log.Println("Transfer manager started")
	return nil
}

// SendFile initiates a file transfer to a peer
func (m *Manager) SendFile(peerID peer.ID, filePath string) (*Transfer, error) {
	log.Printf("📁 SendFile: Starting file transfer to peer %s, file: %s", peerID.String(), filePath)
	
	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("📁 SendFile: Failed to stat file: %v", err)
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	
	log.Printf("📁 SendFile: File info - name: %s, size: %d bytes", fileInfo.Name(), fileInfo.Size())
	
	if fileInfo.Size() > m.maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", fileInfo.Size(), m.maxFileSize)
	}
	
	// Calculate file checksum
	checksum, err := m.calculateChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	
	// Create transfer record
	transfer := &Transfer{
		ID:           fmt.Sprintf("send_%d", time.Now().UnixNano()),
		Filename:     fileInfo.Name(),
		Size:         fileInfo.Size(),
		Status:       StatusPending,
		Direction:    DirectionSend,
		PeerID:       peerID,
		FilePath:     filePath,
		Checksum:     checksum,
		StartTime:    time.Now(),
		lastUpdate:   time.Now(),
	}
	
	// Store transfer
	m.mutex.Lock()
	m.transfers[transfer.ID] = transfer
	m.mutex.Unlock()
	
	// Send transfer offer
	if err := m.sendTransferOffer(transfer); err != nil {
		transfer.Status = StatusFailed
		transfer.Error = err.Error()
		m.notifyTransferUpdate(transfer)
		return nil, fmt.Errorf("failed to send transfer offer: %w", err)
	}
	
	return transfer, nil
}

// AcceptTransfer accepts an incoming file transfer
func (m *Manager) AcceptTransfer(transferID string) error {
	log.Printf("📁 AcceptTransfer: Accepting transfer %s", transferID)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		log.Printf("📁 AcceptTransfer: Transfer not found: %s", transferID)
		return fmt.Errorf("transfer not found: %s", transferID)
	}
	
	if transfer.Direction != DirectionReceive {
		log.Printf("📁 AcceptTransfer: Cannot accept outgoing transfer")
		return fmt.Errorf("cannot accept outgoing transfer")
	}
	
	// Create file for receiving
	filePath := filepath.Join(m.downloadDir, transfer.Filename)
	log.Printf("📁 AcceptTransfer: Creating file at %s", filePath)
	
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("📁 AcceptTransfer: Failed to create file: %v", err)
		return fmt.Errorf("failed to create file: %w", err)
	}
	
	log.Printf("📁 AcceptTransfer: File created successfully")
	
	transfer.file = file
	transfer.FilePath = filePath
	transfer.Status = StatusActive
	transfer.StartTime = time.Now()
	
	// Send acceptance message
	msg := TransferMessage{
		Type: MsgTypeAccept,
		Data: map[string]interface{}{
			"transfer_id": transferID,
		},
	}
	
	log.Printf("📁 AcceptTransfer: Sending acceptance message to peer %s", transfer.PeerID.String())
	if err := m.sendMessage(transfer.PeerID, msg); err != nil {
		log.Printf("📁 AcceptTransfer: Failed to send accept message: %v", err)
		return fmt.Errorf("failed to send accept message: %w", err)
	}
	log.Printf("📁 AcceptTransfer: Acceptance message sent successfully")
	
	m.notifyTransferUpdate(transfer)
	return nil
}

// RejectTransfer rejects an incoming file transfer
func (m *Manager) RejectTransfer(transferID string) error {
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("transfer not found: %s", transferID)
	}
	
	transfer.Status = StatusCancelled
	transfer.EndTime = &time.Time{}
	*transfer.EndTime = time.Now()
	
	// Send rejection message
	msg := TransferMessage{
		Type: MsgTypeReject,
		Data: map[string]interface{}{
			"transfer_id": transferID,
		},
	}
	
	if err := m.sendMessage(transfer.PeerID, msg); err != nil {
		return fmt.Errorf("failed to send reject message: %w", err)
	}
	
	m.notifyTransferUpdate(transfer)
	return nil
}

// CancelTransfer cancels an ongoing transfer
func (m *Manager) CancelTransfer(transferID string) error {
	log.Printf("📁 CancelTransfer: Cancelling transfer %s", transferID)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		log.Printf("📁 CancelTransfer: Transfer not found: %s", transferID)
		return fmt.Errorf("transfer not found: %s", transferID)
	}
	
	if transfer.cancel != nil {
		transfer.cancel()
	}
	
	transfer.Status = StatusCancelled
	transfer.EndTime = &time.Time{}
	*transfer.EndTime = time.Now()
	
	if transfer.file != nil {
		transfer.file.Close()
	}
	
	// Send cancel message
	msg := TransferMessage{
		Type: MsgTypeCancel,
		Data: map[string]interface{}{
			"transfer_id": transferID,
		},
	}
	
	if err := m.sendMessage(transfer.PeerID, msg); err != nil {
		log.Printf("Failed to send cancel message: %v", err)
	}
	
	m.notifyTransferUpdate(transfer)
	return nil
}

// GetTransfers returns all transfers
func (m *Manager) GetTransfers() []*Transfer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	transfers := make([]*Transfer, 0, len(m.transfers))
	for _, transfer := range m.transfers {
		transfers = append(transfers, transfer)
	}
	
	return transfers
}

// GetActiveTransfers returns the number of active transfers
func (m *Manager) GetActiveTransfers() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	count := 0
	for _, transfer := range m.transfers {
		if transfer.Status == StatusActive || transfer.Status == StatusPending {
			count++
		}
	}
	
	return count
}

// SetTransferUpdateHandler sets the callback for transfer updates
func (m *Manager) SetTransferUpdateHandler(handler func(*Transfer)) {
	m.onTransferUpdate = handler
}

// SetTransferOfferHandler sets the callback for transfer offers
func (m *Manager) SetTransferOfferHandler(handler func(*Transfer) bool) {
	m.onTransferOffer = handler
}

// OnPeerConnected handles peer connection events
func (m *Manager) OnPeerConnected(peer *network.Peer) {
	// Implementation for peer connection handling
}

// OnPeerDisconnected handles peer disconnection events
func (m *Manager) OnPeerDisconnected(peerID peer.ID) {
	// Cancel any active transfers with this peer
	m.mutex.RLock()
	var affectedTransfers []*Transfer
	for _, transfer := range m.transfers {
		if transfer.PeerID == peerID && (transfer.Status == StatusActive || transfer.Status == StatusPending) {
			affectedTransfers = append(affectedTransfers, transfer)
		}
	}
	m.mutex.RUnlock()
	
	for _, transfer := range affectedTransfers {
		m.CancelTransfer(transfer.ID)
	}
}

// OnMessage handles incoming messages
func (m *Manager) OnMessage(peerID peer.ID, protocol protocol.ID, data []byte) {
	log.Printf("📁 Transfer OnMessage: protocol=%s, peer=%s, size=%d", protocol, peerID.String(), len(data))
	
	if protocol != network.TransferProtocol {
		log.Printf("📁 Transfer: Ignoring non-transfer protocol: %s", protocol)
		return
	}
	
	var msg TransferMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("📁 Transfer: Failed to unmarshal transfer message: %v", err)
		return
	}
	
	log.Printf("📁 Transfer: Received message type: %s", msg.Type)
	
	switch msg.Type {
	case MsgTypeOffer:
		log.Printf("📁 Transfer: Handling transfer offer")
		m.handleTransferOffer(peerID, msg)
	case MsgTypeAccept:
		log.Printf("📁 Transfer: Handling transfer accept")
		m.handleTransferAccept(peerID, msg)
	case MsgTypeReject:
		log.Printf("📁 Transfer: Handling transfer reject")
		m.handleTransferReject(peerID, msg)
	case MsgTypeData:
		log.Printf("📁 Transfer: Handling transfer data chunk")
		m.handleTransferData(peerID, msg)
	case MsgTypeCancel:
		log.Printf("📁 Transfer: Handling transfer cancel")
		m.handleTransferCancel(peerID, msg)
	case MsgTypeComplete:
		log.Printf("📁 Transfer: Handling transfer complete")
		m.handleTransferComplete(peerID, msg)
	default:
		log.Printf("📁 Transfer: Unknown transfer message type: %s", msg.Type)
	}
}

// sendTransferOffer sends a transfer offer to a peer
func (m *Manager) sendTransferOffer(transfer *Transfer) error {
	msg := TransferMessage{
		Type: MsgTypeOffer,
		Data: map[string]interface{}{
			"transfer_id": transfer.ID,
			"filename":    transfer.Filename,
			"size":        transfer.Size,
			"checksum":    transfer.Checksum,
		},
	}
	
	return m.sendMessage(transfer.PeerID, msg)
}

// sendMessage sends a message to a peer
func (m *Manager) sendMessage(peerID peer.ID, msg TransferMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	return m.network.SendMessage(peerID, network.TransferProtocol, data)
}

// handleTransferOffer handles an incoming transfer offer
func (m *Manager) handleTransferOffer(peerID peer.ID, msg TransferMessage) {
	data := msg.Data
	log.Printf("📁 handleTransferOffer: Received offer from peer %s", peerID.String())
	
	transfer := &Transfer{
		ID:          data["transfer_id"].(string),
		Filename:    data["filename"].(string),
		Size:        int64(data["size"].(float64)),
		Checksum:    data["checksum"].(string),
		Status:      StatusPending,
		Direction:   DirectionReceive,
		PeerID:      peerID,
		StartTime:   time.Now(),
		lastUpdate:  time.Now(),
	}
	
	log.Printf("📁 handleTransferOffer: Transfer details - ID: %s, File: %s, Size: %d", transfer.ID, transfer.Filename, transfer.Size)
	
	// Store transfer
	m.mutex.Lock()
	m.transfers[transfer.ID] = transfer
	m.mutex.Unlock()
	
	// Notify UI
	if m.onTransferOffer != nil {
		log.Printf("📁 handleTransferOffer: Showing transfer offer dialog to user")
		accepted := m.onTransferOffer(transfer)
		log.Printf("📁 handleTransferOffer: User decision: %t", accepted)
		
		if accepted {
			log.Printf("📁 handleTransferOffer: User accepted, calling AcceptTransfer")
			go m.AcceptTransfer(transfer.ID)
		} else {
			log.Printf("📁 handleTransferOffer: User rejected, calling RejectTransfer")
			go m.RejectTransfer(transfer.ID)
		}
	} else {
		log.Printf("📁 handleTransferOffer: No transfer offer handler set!")
	}
}

// handleTransferAccept handles transfer acceptance
func (m *Manager) handleTransferAccept(peerID peer.ID, msg TransferMessage) {
	transferID := msg.Data["transfer_id"].(string)
	log.Printf("📁 handleTransferAccept: Received acceptance for transfer %s from peer %s", transferID, peerID.String())
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		log.Printf("📁 handleTransferAccept: Transfer not found: %s", transferID)
		return
	}
	
	log.Printf("📁 handleTransferAccept: Found transfer, starting file send")
	transfer.Status = StatusActive
	m.notifyTransferUpdate(transfer)
	
	// Start sending file
	go m.sendFile(transfer)
}

// handleTransferReject handles transfer rejection
func (m *Manager) handleTransferReject(peerID peer.ID, msg TransferMessage) {
	transferID := msg.Data["transfer_id"].(string)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	transfer.Status = StatusCancelled
	transfer.EndTime = &time.Time{}
	*transfer.EndTime = time.Now()
	
	m.notifyTransferUpdate(transfer)
}

// handleTransferCancel handles transfer cancellation
func (m *Manager) handleTransferCancel(peerID peer.ID, msg TransferMessage) {
	transferID := msg.Data["transfer_id"].(string)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	if transfer.cancel != nil {
		transfer.cancel()
	}
	
	transfer.Status = StatusCancelled
	transfer.EndTime = &time.Time{}
	*transfer.EndTime = time.Now()
	
	if transfer.file != nil {
		transfer.file.Close()
	}
	
	m.notifyTransferUpdate(transfer)
}

// handleTransferComplete handles transfer completion
func (m *Manager) handleTransferComplete(peerID peer.ID, msg TransferMessage) {
	transferID := msg.Data["transfer_id"].(string)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	transfer.Status = StatusCompleted
	transfer.Progress = 100.0
	transfer.EndTime = &time.Time{}
	*transfer.EndTime = time.Now()
	
	if transfer.file != nil {
		transfer.file.Close()
	}
	
	m.notifyTransferUpdate(transfer)
}

// sendFile sends a file to a peer
func (m *Manager) sendFile(transfer *Transfer) {
	log.Printf("📁 sendFile: Starting to send file %s to peer %s", transfer.Filename, transfer.PeerID.String())
	
	file, err := os.Open(transfer.FilePath)
	if err != nil {
		log.Printf("📁 sendFile: Failed to open file: %v", err)
		transfer.Status = StatusFailed
		transfer.Error = err.Error()
		m.notifyTransferUpdate(transfer)
		return
	}
	defer file.Close()
	
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("📁 sendFile: Failed to stat file: %v", err)
		transfer.Status = StatusFailed
		transfer.Error = err.Error()
		m.notifyTransferUpdate(transfer)
		return
	}
	
	fileSize := fileInfo.Size()
	log.Printf("📁 sendFile: File size: %d bytes", fileSize)
	
	// Send file in chunks
	const chunkSize = 4 * 1024 // 4KB chunks (smaller for debugging)
	buffer := make([]byte, chunkSize)
	var totalSent int64 = 0
	chunkIndex := 0
	
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("📁 sendFile: Failed to read file: %v", err)
			transfer.Status = StatusFailed
			transfer.Error = err.Error()
			m.notifyTransferUpdate(transfer)
			return
		}
		
		if bytesRead == 0 {
			break // End of file
		}
		
		// Send this chunk
		chunk := buffer[:bytesRead]
		if err := m.sendFileChunk(transfer, chunkIndex, chunk, totalSent+int64(bytesRead) == fileSize); err != nil {
			log.Printf("📁 sendFile: Failed to send chunk %d: %v", chunkIndex, err)
			transfer.Status = StatusFailed
			transfer.Error = err.Error()
			m.notifyTransferUpdate(transfer)
			return
		}
		
		totalSent += int64(bytesRead)
		transfer.Transferred = totalSent
		transfer.Progress = float64(totalSent) * 100.0 / float64(fileSize)
		
		log.Printf("📁 sendFile: Sent chunk %d, %d bytes, progress: %.1f%%", chunkIndex, bytesRead, transfer.Progress)
		
		// Update progress
		m.notifyTransferUpdate(transfer)
		
		chunkIndex++
	}
	
	log.Printf("📁 sendFile: File transfer completed, total sent: %d bytes", totalSent)
	transfer.Status = StatusCompleted
	transfer.Progress = 100.0
	now := time.Now()
	transfer.EndTime = &now
	
	m.notifyTransferUpdate(transfer)
}

// calculateChecksum calculates SHA256 checksum of a file
func (m *Manager) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// notifyTransferUpdate notifies about transfer updates
func (m *Manager) notifyTransferUpdate(transfer *Transfer) {
	if m.onTransferUpdate != nil {
		m.onTransferUpdate(transfer)
	}
}

// sendFileChunk sends a file chunk to a peer
func (m *Manager) sendFileChunk(transfer *Transfer, chunkIndex int, data []byte, isLast bool) error {
	log.Printf("📁 sendFileChunk: Sending chunk %d, size: %d bytes, isLast: %t", chunkIndex, len(data), isLast)
	
	// Encode data as base64 for JSON transport
	encodedData := base64.StdEncoding.EncodeToString(data)
	log.Printf("📁 sendFileChunk: Encoded data size: %d characters", len(encodedData))
	
	msg := TransferMessage{
		Type: MsgTypeData,
		Data: map[string]interface{}{
			"transfer_id":  transfer.ID,
			"chunk_index":  chunkIndex,
			"data":         encodedData,
			"is_last":      isLast,
		},
	}
	
	log.Printf("📁 sendFileChunk: Sending message to peer %s", transfer.PeerID.String())
	err := m.sendMessage(transfer.PeerID, msg)
	if err != nil {
		log.Printf("📁 sendFileChunk: Failed to send chunk %d: %v", chunkIndex, err)
	} else {
		log.Printf("📁 sendFileChunk: Successfully sent chunk %d", chunkIndex)
	}
	return err
}

// handleTransferData handles incoming file data chunks
func (m *Manager) handleTransferData(peerID peer.ID, msg TransferMessage) {
	data := msg.Data
	transferID := data["transfer_id"].(string)
	chunkIndex := int(data["chunk_index"].(float64))
	encodedData := data["data"].(string)
	isLast := data["is_last"].(bool)
	
	// Decode base64 data
	chunkData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		log.Printf("📁 handleTransferData: Failed to decode chunk data: %v", err)
		return
	}
	
	log.Printf("📁 handleTransferData: Received chunk %d, size: %d bytes, isLast: %t", chunkIndex, len(chunkData), isLast)
	
	m.mutex.RLock()
	transfer, exists := m.transfers[transferID]
	m.mutex.RUnlock()
	
	if !exists {
		log.Printf("📁 handleTransferData: Transfer not found: %s", transferID)
		return
	}
	
	if transfer.file == nil {
		log.Printf("📁 handleTransferData: No file handle for transfer: %s", transferID)
		return
	}
	
	// Write chunk to file
	bytesWritten, err := transfer.file.Write(chunkData)
	if err != nil {
		log.Printf("📁 handleTransferData: Failed to write chunk: %v", err)
		transfer.Status = StatusFailed
		transfer.Error = err.Error()
		transfer.file.Close()
		transfer.file = nil
		m.notifyTransferUpdate(transfer)
		return
	}
	
	transfer.Transferred += int64(bytesWritten)
	transfer.Progress = float64(transfer.Transferred) * 100.0 / float64(transfer.Size)
	
	log.Printf("📁 handleTransferData: Wrote %d bytes, total: %d/%d, progress: %.1f%%", 
		bytesWritten, transfer.Transferred, transfer.Size, transfer.Progress)
	
	m.notifyTransferUpdate(transfer)
	
	// If this is the last chunk, complete the transfer
	if isLast {
		log.Printf("📁 handleTransferData: Transfer completed: %s", transferID)
		transfer.Status = StatusCompleted
		transfer.Progress = 100.0
		now := time.Now()
		transfer.EndTime = &now
		
		transfer.file.Close()
		transfer.file = nil
		
		m.notifyTransferUpdate(transfer)
	}
}