package network

import (
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// networkNotifiee implements the network.Notifiee interface
type networkNotifiee Manager

// Listen is called when we start listening on a new address
func (nn *networkNotifiee) Listen(n network.Network, addr multiaddr.Multiaddr) {
	log.Printf("Listening on %s", addr)
}

// ListenClose is called when we stop listening on an address
func (nn *networkNotifiee) ListenClose(n network.Network, addr multiaddr.Multiaddr) {
	log.Printf("Stopped listening on %s", addr)
}

// Connected is called when we connect to a peer
func (nn *networkNotifiee) Connected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	log.Printf("🔗 PEER CONNECTED: %s", peerID.String())
	log.Printf("  Remote address: %s", conn.RemoteMultiaddr().String())
	log.Printf("  Local address: %s", conn.LocalMultiaddr().String())

	// Create peer info
	peer := &Peer{
		ID:          peerID.String(),
		Nickname:    peerID.String()[:8], // Default nickname, will be updated
		ConnectedAt: time.Now(),
		PeerID:      peerID,
		Addresses:   []multiaddr.Multiaddr{conn.RemoteMultiaddr()},
	}

	// Add to peers map (check for duplicates)
	manager := (*Manager)(nn)
	manager.peersMutex.Lock()

	// Check if peer already exists
	if existingPeer, exists := manager.peers[peerID]; exists {
		log.Printf("  Peer already exists, updating connection info")
		// Update existing peer with new address
		existingPeer.Addresses = append(existingPeer.Addresses, conn.RemoteMultiaddr())
		manager.peersMutex.Unlock()
		log.Printf("  Updated existing peer, no new chat notification needed")
		return
	}

	// Add new peer
	manager.peers[peerID] = peer
	totalPeers := len(manager.peers)
	manager.peersMutex.Unlock()

	log.Printf("  Total peers now: %d", totalPeers)

	// Notify handlers (only for new peers)
	manager.notifyPeerConnected(peer)

	log.Printf("  New peer added to chat system")
}

// Disconnected is called when we disconnect from a peer
func (nn *networkNotifiee) Disconnected(n network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	log.Printf("🔗 PEER DISCONNECTED: %s (connection: %s)", peerID.String(), conn.RemoteMultiaddr().String())

	manager := (*Manager)(nn)

	// Check if we still have other connections to this peer
	if manager.host.Network().Connectedness(peerID) == network.Connected {
		log.Printf("🔗 Still have other connections to peer %s, keeping peer info", peerID.String())
		return
	}

	// Remove from peers map only if completely disconnected
	manager.peersMutex.Lock()
	delete(manager.peers, peerID)
	peerCount := len(manager.peers)
	manager.peersMutex.Unlock()

	log.Printf("🔗 Peer %s fully disconnected, total peers: %d", peerID.String(), peerCount)

	// Notify handlers
	manager.notifyPeerDisconnected(peerID)
}

// discoveryNotifiee implements the mdns.Notifiee interface for mDNS discovery
type discoveryNotifiee struct {
	manager *Manager
}

// HandlePeerFound is called when a peer is discovered via mDNS
func (dn *discoveryNotifiee) HandlePeerFound(peerInfo peer.AddrInfo) {
	log.Printf("🔍 mDNS Discovery: Found peer %s", peerInfo.ID.String())
	log.Printf("  Peer addresses: %v", peerInfo.Addrs)

	// Don't connect to ourselves
	if peerInfo.ID == dn.manager.host.ID() {
		log.Printf("  Skipping self-connection")
		return
	}

	// Connect to the discovered peer
	log.Printf("  Attempting connection to peer %s...", peerInfo.ID.String())
	if err := dn.manager.host.Connect(dn.manager.ctx, peerInfo); err != nil {
		log.Printf("  ❌ Failed to connect to discovered peer %s: %v", peerInfo.ID, err)
	} else {
		log.Printf("  ✅ Successfully connected to peer %s via mDNS", peerInfo.ID.String())
	}
}
