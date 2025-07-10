// Package chat provides real-time P2P chat functionality
package chat

import (
	"encoding/json"
	"fmt"
	"log"
	"shario/internal/network"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Sender    string    `json:"sender"`
	SenderID  peer.ID   `json:"sender_id"`
	Timestamp time.Time `json:"timestamp"`
	RoomID    string    `json:"room_id"`
	Type      string    `json:"type"` // "text", "system", "file"
}

// Room represents a chat room
type Room struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Type        string             `json:"type"` // "direct", "group"
	Participants map[peer.ID]string `json:"participants"`
	Messages    []*Message         `json:"messages"`
	CreatedAt   time.Time          `json:"created_at"`
	LastMessage *Message           `json:"last_message,omitempty"`
	UnreadCount int                `json:"unread_count"`
	mutex       sync.RWMutex
}

// ChatMessage represents a chat protocol message
type ChatMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// Message types
const (
	MsgTypeText           = "text"
	MsgTypeSystem         = "system"
	MsgTypeJoin           = "join"
	MsgTypeLeave          = "leave"
	MsgTypeTyping         = "typing"
	MsgTypeNicknameChange = "nickname_change"
)

// Manager handles chat functionality
type Manager struct {
	network *network.Manager
	rooms   map[string]*Room
	mutex   sync.RWMutex
	
	// Global room
	globalRoom *Room
	
	// Current user info
	nickname string
	
	// Event handlers
	onMessageReceived func(*Message)
	onRoomUpdated     func(*Room)
	onTypingIndicator func(roomID string, senderID peer.ID, isTyping bool)
}

// New creates a new chat manager
func New(networkMgr *network.Manager) *Manager {
	mgr := &Manager{
		network: networkMgr,
		rooms:   make(map[string]*Room),
	}
	
	// Register as network event handler
	networkMgr.AddEventHandler("chat", mgr)
	
	return mgr
}

// Start initializes the chat manager
func (m *Manager) Start() error {
	log.Println("Chat manager started")
	
	// Create global chat room
	m.createGlobalRoom()
	
	return nil
}

// SendMessage sends a chat message to a room
func (m *Manager) SendMessage(roomID, content string) error {
	m.mutex.RLock()
	room, exists := m.rooms[roomID]
	m.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("room not found: %s", roomID)
	}
	
	message := &Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		Content:   content,
		Sender:    m.nickname,
		SenderID:  m.network.GetHost().ID(),
		Timestamp: time.Now(),
		RoomID:    roomID,
		Type:      MsgTypeText,
	}
	
	// Add to room
	room.mutex.Lock()
	room.Messages = append(room.Messages, message)
	room.LastMessage = message
	room.mutex.Unlock()
	
	// Send to all participants (except for local test rooms)
	if room.Type != "local_test" {
		participantCount := 0
		for peerID := range room.Participants {
			if peerID != m.network.GetHost().ID() {
				participantCount++
				log.Printf("📤 Sending message to peer: %s", peerID.String())
				go m.sendMessageToPeer(peerID, message)
			}
		}
		log.Printf("📤 Message sent to %d participants in room '%s'", participantCount, room.Name)
	} else {
		log.Printf("📝 Local test message (not sent to network)")
	}
	
	// Notify handlers
	if m.onMessageReceived != nil {
		go m.onMessageReceived(message)
	}
	
	if m.onRoomUpdated != nil {
		go m.onRoomUpdated(room)
	}
	
	return nil
}

// CreateDirectRoom creates a direct chat room with a peer
func (m *Manager) CreateDirectRoom(peerID peer.ID, peerNickname string) *Room {
	roomID := m.generateDirectRoomID(m.network.GetHost().ID(), peerID)
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Check if room already exists
	if room, exists := m.rooms[roomID]; exists {
		return room
	}
	
	room := &Room{
		ID:   roomID,
		Name: peerNickname,
		Type: "direct",
		Participants: map[peer.ID]string{
			m.network.GetHost().ID(): m.nickname,
			peerID:                   peerNickname,
		},
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now(),
	}
	
	m.rooms[roomID] = room
	
	// Send join message to peer
	m.sendJoinMessage(peerID, room)
	
	return room
}

// CreateLocalTestRoom creates a local-only test room that doesn't send to peers
func (m *Manager) CreateLocalTestRoom(roomName string) *Room {
	roomID := fmt.Sprintf("local_test_%d", time.Now().UnixNano())
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	room := &Room{
		ID:   roomID,
		Name: roomName,
		Type: "local_test",
		Participants: map[peer.ID]string{
			m.network.GetHost().ID(): m.nickname,
		},
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now(),
	}
	
	m.rooms[roomID] = room
	
	// Add a welcome message
	welcomeMsg := &Message{
		ID:        fmt.Sprintf("welcome_%d", time.Now().UnixNano()),
		Content:   "Welcome to the test room! This is a local-only room for testing chat functionality.",
		Sender:    "System",
		SenderID:  "",
		Timestamp: time.Now(),
		RoomID:    roomID,
		Type:      MsgTypeSystem,
	}
	
	room.Messages = append(room.Messages, welcomeMsg)
	room.LastMessage = welcomeMsg
	
	return room
}

// GetRooms returns all chat rooms
func (m *Manager) GetRooms() []*Room {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	
	return rooms
}

// GetRoom returns a specific room by ID
func (m *Manager) GetRoom(roomID string) (*Room, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	room, exists := m.rooms[roomID]
	return room, exists
}

// GetActiveRooms returns the number of active rooms
func (m *Manager) GetActiveRooms() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return len(m.rooms)
}

// GetGlobalRoom returns the global chat room
func (m *Manager) GetGlobalRoom() *Room {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return m.globalRoom
}

// SetNickname sets the user's nickname and broadcasts the change
func (m *Manager) SetNickname(nickname string) {
	oldNickname := m.nickname
	m.nickname = nickname
	
	log.Printf("🎭 Chat SetNickname: '%s' -> '%s'", oldNickname, nickname)
	
	// If nickname actually changed, broadcast it to all peers
	if oldNickname != "" && oldNickname != nickname {
		log.Printf("🎭 Chat SetNickname: Broadcasting change '%s' -> '%s'", oldNickname, nickname)
		m.broadcastNicknameChange(oldNickname, nickname)
	} else {
		log.Printf("🎭 Chat SetNickname: No broadcast needed (oldNickname='%s', nickname='%s')", oldNickname, nickname)
	}
}

// GetNickname returns the user's nickname
func (m *Manager) GetNickname() string {
	return m.nickname
}

// SetMessageHandler sets the callback for received messages
func (m *Manager) SetMessageHandler(handler func(*Message)) {
	m.onMessageReceived = handler
}

// SetRoomUpdateHandler sets the callback for room updates
func (m *Manager) SetRoomUpdateHandler(handler func(*Room)) {
	m.onRoomUpdated = handler
}

// SetTypingIndicatorHandler sets the callback for typing indicators
func (m *Manager) SetTypingIndicatorHandler(handler func(roomID string, senderID peer.ID, isTyping bool)) {
	m.onTypingIndicator = handler
}

// SendTypingIndicator sends a typing indicator
func (m *Manager) SendTypingIndicator(roomID string, isTyping bool) {
	m.mutex.RLock()
	room, exists := m.rooms[roomID]
	m.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	msg := ChatMessage{
		Type: MsgTypeTyping,
		Data: map[string]interface{}{
			"room_id":   roomID,
			"is_typing": isTyping,
		},
	}
	
	// Send to all participants
	for peerID := range room.Participants {
		if peerID != m.network.GetHost().ID() {
			go m.sendChatMessage(peerID, msg)
		}
	}
}

// MarkRoomAsRead marks a room as read
func (m *Manager) MarkRoomAsRead(roomID string) {
	m.mutex.RLock()
	room, exists := m.rooms[roomID]
	m.mutex.RUnlock()
	
	if !exists {
		return
	}
	
	room.mutex.Lock()
	room.UnreadCount = 0
	room.mutex.Unlock()
	
	if m.onRoomUpdated != nil {
		go m.onRoomUpdated(room)
	}
}

// createGlobalRoom creates the global chat room for all users
func (m *Manager) createGlobalRoom() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	globalRoom := &Room{
		ID:   "global",
		Name: "Global Chat",
		Type: "global",
		Participants: map[peer.ID]string{
			m.network.GetHost().ID(): m.nickname,
		},
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now(),
	}
	
	// Add welcome message
	welcomeMsg := &Message{
		ID:        fmt.Sprintf("welcome_%d", time.Now().UnixNano()),
		Content:   fmt.Sprintf("Welcome to Shario! %s joined the global chat.", m.nickname),
		Sender:    "System",
		SenderID:  "",
		Timestamp: time.Now(),
		RoomID:    "global",
		Type:      MsgTypeSystem,
	}
	
	globalRoom.Messages = append(globalRoom.Messages, welcomeMsg)
	globalRoom.LastMessage = welcomeMsg
	
	m.rooms["global"] = globalRoom
	m.globalRoom = globalRoom
	
	log.Printf("Created global chat room")
}

// OnPeerConnected handles peer connection events
func (m *Manager) OnPeerConnected(peer *network.Peer) {
	log.Printf("Chat: Peer connected: %s", peer.ID)
	
	// Add peer to global room
	m.addPeerToGlobalRoom(peer)
}

// addPeerToGlobalRoom adds a newly connected peer to the global room
func (m *Manager) addPeerToGlobalRoom(peer *network.Peer) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.globalRoom == nil {
		return
	}
	
	// Check if peer is already in the global room
	if _, exists := m.globalRoom.Participants[peer.PeerID]; exists {
		log.Printf("Peer %s already in global chat, skipping duplicate addition", peer.Nickname)
		return
	}
	
	// Add peer to global room participants
	m.globalRoom.Participants[peer.PeerID] = peer.Nickname
	
	// Add system message about peer joining
	joinMsg := &Message{
		ID:        fmt.Sprintf("join_%d", time.Now().UnixNano()),
		Content:   fmt.Sprintf("%s joined the global chat", peer.Nickname),
		Sender:    "System",
		SenderID:  "",
		Timestamp: time.Now(),
		RoomID:    "global",
		Type:      MsgTypeSystem,
	}
	
	m.globalRoom.Messages = append(m.globalRoom.Messages, joinMsg)
	m.globalRoom.LastMessage = joinMsg
	
	// Notify UI to refresh
	if m.onMessageReceived != nil {
		go m.onMessageReceived(joinMsg)
	}
	
	if m.onRoomUpdated != nil {
		go m.onRoomUpdated(m.globalRoom)
	}
	
	log.Printf("Added peer %s to global chat", peer.Nickname)
}

// OnPeerDisconnected handles peer disconnection events
func (m *Manager) OnPeerDisconnected(peerID peer.ID) {
	log.Printf("Chat: Peer disconnected: %s", peerID)
	
	// Add system message to rooms with this peer
	m.mutex.RLock()
	var affectedRooms []*Room
	for _, room := range m.rooms {
		if _, exists := room.Participants[peerID]; exists {
			affectedRooms = append(affectedRooms, room)
		}
	}
	m.mutex.RUnlock()
	
	for _, room := range affectedRooms {
		m.addSystemMessage(room, fmt.Sprintf("%s has disconnected", room.Participants[peerID]))
	}
}

// OnMessage handles incoming messages
func (m *Manager) OnMessage(peerID peer.ID, protocol protocol.ID, data []byte) {
	if protocol != network.ChatProtocol {
		return
	}
	
	log.Printf("📥 Received message from peer %s, size: %d bytes", peerID.String(), len(data))
	
	var msg ChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal chat message: %v", err)
		return
	}
	
	log.Printf("📥 Message type: %s", msg.Type)
	
	switch msg.Type {
	case MsgTypeText:
		m.handleTextMessage(peerID, msg)
	case MsgTypeSystem:
		m.handleSystemMessage(peerID, msg)
	case MsgTypeJoin:
		m.handleJoinMessage(peerID, msg)
	case MsgTypeLeave:
		m.handleLeaveMessage(peerID, msg)
	case MsgTypeTyping:
		m.handleTypingIndicator(peerID, msg)
	case MsgTypeNicknameChange:
		m.handleNicknameChange(peerID, msg)
	default:
		log.Printf("Unknown chat message type: %s", msg.Type)
	}
}

// sendMessageToPeer sends a message to a specific peer
func (m *Manager) sendMessageToPeer(peerID peer.ID, message *Message) {
	msg := ChatMessage{
		Type: MsgTypeText,
		Data: map[string]interface{}{
			"id":        message.ID,
			"content":   message.Content,
			"sender":    message.Sender,
			"sender_id": message.SenderID.String(),
			"timestamp": message.Timestamp.Unix(),
			"room_id":   message.RoomID,
			"type":      message.Type,
		},
	}
	
	m.sendChatMessage(peerID, msg)
}

// sendChatMessage sends a chat message to a peer
func (m *Manager) sendChatMessage(peerID peer.ID, msg ChatMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal chat message: %v", err)
		return
	}
	
	if err := m.network.SendMessage(peerID, network.ChatProtocol, data); err != nil {
		log.Printf("Failed to send chat message to peer %s: %v", peerID, err)
	}
}

// sendJoinMessage sends a join message to a peer
func (m *Manager) sendJoinMessage(peerID peer.ID, room *Room) {
	msg := ChatMessage{
		Type: MsgTypeJoin,
		Data: map[string]interface{}{
			"room_id":      room.ID,
			"room_name":    room.Name,
			"room_type":    room.Type,
			"created_at":   room.CreatedAt.Unix(),
			"participants": m.serializeParticipants(room.Participants),
		},
	}
	
	m.sendChatMessage(peerID, msg)
}

// handleTextMessage handles incoming text messages
func (m *Manager) handleTextMessage(peerID peer.ID, msg ChatMessage) {
	data := msg.Data
	
	senderID, err := peer.Decode(data["sender_id"].(string))
	if err != nil {
		log.Printf("Failed to decode sender ID: %v", err)
		return
	}
	
	// Get the current nickname for this peer (not the one sent in the message)
	currentNickname := m.getCurrentPeerNickname(senderID)
	sentNickname := data["sender"].(string)
	
	if currentNickname == "" {
		currentNickname = sentNickname // fallback to sent nickname
		log.Printf("📥 Using sent nickname '%s' for peer %s (no current nickname found)", sentNickname, senderID.String())
	} else if currentNickname != sentNickname {
		log.Printf("📥 Updated nickname for peer %s: sent='%s' current='%s'", senderID.String(), sentNickname, currentNickname)
	}
	
	message := &Message{
		ID:        data["id"].(string),
		Content:   data["content"].(string),
		Sender:    currentNickname,
		SenderID:  senderID,
		Timestamp: time.Unix(int64(data["timestamp"].(float64)), 0),
		RoomID:    data["room_id"].(string),
		Type:      data["type"].(string),
	}
	
	// Find or create room
	m.mutex.RLock()
	room, exists := m.rooms[message.RoomID]
	m.mutex.RUnlock()
	
	if !exists {
		// Create new room
		room = &Room{
			ID:   message.RoomID,
			Name: message.Sender,
			Type: "direct",
			Participants: map[peer.ID]string{
				m.network.GetHost().ID(): m.nickname,
				peerID:                   message.Sender,
			},
			Messages:  make([]*Message, 0),
			CreatedAt: time.Now(),
		}
		
		m.mutex.Lock()
		m.rooms[message.RoomID] = room
		m.mutex.Unlock()
	}
	
	// Add message to room
	room.mutex.Lock()
	room.Messages = append(room.Messages, message)
	room.LastMessage = message
	room.UnreadCount++
	room.mutex.Unlock()
	
	// Notify handlers
	if m.onMessageReceived != nil {
		go m.onMessageReceived(message)
	}
	
	if m.onRoomUpdated != nil {
		go m.onRoomUpdated(room)
	}
}

// handleSystemMessage handles system messages
func (m *Manager) handleSystemMessage(peerID peer.ID, msg ChatMessage) {
	// TODO: Implement system message handling
}

// handleJoinMessage handles join messages
func (m *Manager) handleJoinMessage(peerID peer.ID, msg ChatMessage) {
	data := msg.Data
	roomID := data["room_id"].(string)
	
	// If this is for the global room, just add the peer to existing global room
	if roomID == "global" {
		log.Printf("Received global room join message from peer %s", peerID.String())
		
		// Get peer nickname from network manager
		if peers := m.network.GetPeers(); len(peers) > 0 {
			for _, peer := range peers {
				if peer.PeerID == peerID {
					m.addPeerToGlobalRoom(peer)
					break
				}
			}
		}
		return
	}
	
	// Handle other room types (direct rooms, etc.)
	room := &Room{
		ID:        roomID,
		Name:      data["room_name"].(string),
		Type:      data["room_type"].(string),
		CreatedAt: time.Unix(int64(data["created_at"].(float64)), 0),
		Messages:  make([]*Message, 0),
	}
	
	// Deserialize participants
	participants := data["participants"].(map[string]interface{})
	room.Participants = make(map[peer.ID]string)
	for idStr, nickname := range participants {
		if id, err := peer.Decode(idStr); err == nil {
			room.Participants[id] = nickname.(string)
		}
	}
	
	m.mutex.Lock()
	m.rooms[room.ID] = room
	m.mutex.Unlock()
	
	// Add system message
	m.addSystemMessage(room, fmt.Sprintf("Joined chat with %s", room.Participants[peerID]))
	
	if m.onRoomUpdated != nil {
		go m.onRoomUpdated(room)
	}
}

// handleLeaveMessage handles leave messages
func (m *Manager) handleLeaveMessage(peerID peer.ID, msg ChatMessage) {
	// TODO: Implement leave message handling
}

// handleTypingIndicator handles typing indicators
func (m *Manager) handleTypingIndicator(peerID peer.ID, msg ChatMessage) {
	data := msg.Data
	roomID := data["room_id"].(string)
	isTyping := data["is_typing"].(bool)
	
	if m.onTypingIndicator != nil {
		go m.onTypingIndicator(roomID, peerID, isTyping)
	}
}

// broadcastNicknameChange sends nickname change to all connected peers
func (m *Manager) broadcastNicknameChange(oldNickname, newNickname string) {
	log.Printf("🔄 Broadcasting nickname change: %s -> %s", oldNickname, newNickname)
	
	// Get all connected peers
	peers := m.network.GetPeers()
	if len(peers) == 0 {
		log.Printf("🔄 No peers to notify of nickname change")
		return
	}
	
	log.Printf("🔄 Notifying %d peers of nickname change", len(peers))
	
	// Create nickname change message
	msg := ChatMessage{
		Type: MsgTypeNicknameChange,
		Data: map[string]interface{}{
			"old_nickname": oldNickname,
			"new_nickname": newNickname,
			"peer_id":      m.network.GetHost().ID().String(),
		},
	}
	
	// Send to all peers
	for _, peer := range peers {
		log.Printf("🔄 Sending nickname change to peer %s", peer.Nickname)
		go m.sendChatMessage(peer.PeerID, msg)
	}
	
	// Update global room participants
	m.updateNicknameInRooms(oldNickname, newNickname)
	log.Printf("🔄 Nickname change broadcast complete")
}

// handleNicknameChange handles incoming nickname change notifications
func (m *Manager) handleNicknameChange(peerID peer.ID, msg ChatMessage) {
	data := msg.Data
	oldNickname := data["old_nickname"].(string)
	newNickname := data["new_nickname"].(string)
	
	log.Printf("📥 Nickname change from peer %s: %s -> %s", peerID.String(), oldNickname, newNickname)
	
	// Update peer nickname in network manager
	m.updatePeerNickname(peerID, newNickname)
	
	// Update nickname in all rooms for this specific peer
	m.updatePeerNicknameInRooms(peerID, newNickname)
	
	// Add system message to global room
	if m.globalRoom != nil {
		systemMsg := &Message{
			ID:        fmt.Sprintf("nick_change_%d", time.Now().UnixNano()),
			Content:   fmt.Sprintf("%s changed their nickname to %s", oldNickname, newNickname),
			Sender:    "System",
			SenderID:  "",
			Timestamp: time.Now(),
			RoomID:    "global",
			Type:      MsgTypeSystem,
		}
		
		m.globalRoom.mutex.Lock()
		m.globalRoom.Messages = append(m.globalRoom.Messages, systemMsg)
		m.globalRoom.LastMessage = systemMsg
		m.globalRoom.mutex.Unlock()
		
		// Notify UI
		if m.onMessageReceived != nil {
			go m.onMessageReceived(systemMsg)
		}
		
		if m.onRoomUpdated != nil {
			go m.onRoomUpdated(m.globalRoom)
		}
	}
}

// updatePeerNickname updates a peer's nickname in the network manager
func (m *Manager) updatePeerNickname(peerID peer.ID, newNickname string) {
	peers := m.network.GetPeers()
	for _, peer := range peers {
		if peer.PeerID == peerID {
			peer.Nickname = newNickname
			log.Printf("Updated peer %s nickname to %s", peerID.String(), newNickname)
			break
		}
	}
}

// updateNicknameInRooms updates nickname in all rooms where the peer participates
func (m *Manager) updateNicknameInRooms(oldNickname, newNickname string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	updatedRooms := 0
	
	for _, room := range m.rooms {
		room.mutex.Lock()
		// Update participant nickname for any peer with the old nickname
		for peerID, nickname := range room.Participants {
			if nickname == oldNickname {
				room.Participants[peerID] = newNickname
				updatedRooms++
				break
			}
		}
		room.mutex.Unlock()
	}
	
	log.Printf("Updated nickname in %d rooms", updatedRooms)
}

// updatePeerNicknameInRooms updates a specific peer's nickname in all rooms
func (m *Manager) updatePeerNicknameInRooms(peerID peer.ID, newNickname string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	updatedRooms := 0
	
	for _, room := range m.rooms {
		room.mutex.Lock()
		if _, exists := room.Participants[peerID]; exists {
			room.Participants[peerID] = newNickname
			updatedRooms++
		}
		room.mutex.Unlock()
	}
	
	log.Printf("Updated peer %s nickname to %s in %d rooms", peerID.String(), newNickname, updatedRooms)
	
	// Refresh UI
	if m.onRoomUpdated != nil && m.globalRoom != nil {
		go m.onRoomUpdated(m.globalRoom)
	}
}

// getCurrentPeerNickname gets the current nickname for a peer
func (m *Manager) getCurrentPeerNickname(peerID peer.ID) string {
	// First check network manager for peer info
	peers := m.network.GetPeers()
	for _, peer := range peers {
		if peer.PeerID == peerID {
			log.Printf("🔍 Found peer %s with nickname '%s' in network manager", peerID.String(), peer.Nickname)
			return peer.Nickname
		}
	}
	
	// Fallback: check global room participants
	if m.globalRoom != nil {
		m.globalRoom.mutex.RLock()
		nickname, exists := m.globalRoom.Participants[peerID]
		m.globalRoom.mutex.RUnlock()
		if exists {
			log.Printf("🔍 Found peer %s with nickname '%s' in global room", peerID.String(), nickname)
			return nickname
		}
	}
	
	// No nickname found
	log.Printf("🔍 No current nickname found for peer %s", peerID.String())
	return ""
}

// addSystemMessage adds a system message to a room
func (m *Manager) addSystemMessage(room *Room, content string) {
	message := &Message{
		ID:        fmt.Sprintf("sys_%d", time.Now().UnixNano()),
		Content:   content,
		Sender:    "System",
		SenderID:  "",
		Timestamp: time.Now(),
		RoomID:    room.ID,
		Type:      MsgTypeSystem,
	}
	
	room.mutex.Lock()
	room.Messages = append(room.Messages, message)
	room.LastMessage = message
	room.mutex.Unlock()
	
	if m.onMessageReceived != nil {
		go m.onMessageReceived(message)
	}
}

// generateDirectRoomID generates a consistent room ID for direct messages
func (m *Manager) generateDirectRoomID(peer1, peer2 peer.ID) string {
	if peer1.String() < peer2.String() {
		return fmt.Sprintf("direct_%s_%s", peer1.String(), peer2.String())
	}
	return fmt.Sprintf("direct_%s_%s", peer2.String(), peer1.String())
}

// serializeParticipants serializes participants map for JSON
func (m *Manager) serializeParticipants(participants map[peer.ID]string) map[string]interface{} {
	result := make(map[string]interface{})
	for id, nickname := range participants {
		result[id.String()] = nickname
	}
	return result
}