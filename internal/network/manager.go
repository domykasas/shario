// Package network provides P2P networking functionality using libp2p
package network

import (
	"context"
	"fmt"
	"log"
	"shario/internal/identity"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/multiformats/go-multiaddr"
)

const (
	// Protocol IDs
	ChatProtocol     = protocol.ID("/shario/chat/1.0.0")
	TransferProtocol = protocol.ID("/shario/transfer/1.0.0")
	
	// Discovery constants
	ServiceTag = "shario-p2p"
	
	// Connection timeouts
	ConnectionTimeout = 30 * time.Second
)

// Peer represents a connected peer
type Peer struct {
	ID          string
	Nickname    string
	ConnectedAt time.Time
	PeerID      peer.ID
	Addresses   []multiaddr.Multiaddr
}

// Manager handles all P2P networking operations
type Manager struct {
	// Core components
	host         host.Host
	dht          *dht.IpfsDHT
	discovery    mdns.Service
	routingDisc  *routing.RoutingDiscovery
	identity     *identity.Manager
	
	// State management
	ctx           context.Context
	cancel        context.CancelFunc
	peers         map[peer.ID]*Peer
	peersMutex    sync.RWMutex
	eventHandlers map[string][]NetworkEventHandler
	handlersMutex sync.RWMutex
	
	// Configuration
	listenAddrs   []multiaddr.Multiaddr
	bootstrapPeers []peer.AddrInfo
}

// NetworkEventHandler defines the interface for network event callbacks
type NetworkEventHandler interface {
	OnPeerConnected(peer *Peer)
	OnPeerDisconnected(peerID peer.ID)
	OnMessage(peerID peer.ID, protocol protocol.ID, data []byte)
}

// New creates a new network manager
func New(ctx context.Context, identityMgr *identity.Manager) (*Manager, error) {
	netCtx, cancel := context.WithCancel(ctx)
	
	// Create listen addresses
	listenAddrs := []multiaddr.Multiaddr{
		multiaddr.StringCast("/ip4/0.0.0.0/tcp/0"),
		multiaddr.StringCast("/ip6/::/tcp/0"),
	}
	
	// Create libp2p host
	h, err := libp2p.New(
		libp2p.Identity(identityMgr.GetPrivateKey()),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.NATPortMap(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{}),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}
	
	log.Printf("Libp2p host created with ID: %s", h.ID().String())
	
	// Create DHT
	kademliaDHT, err := dht.New(netCtx, h)
	if err != nil {
		cancel()
		h.Close()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}
	
	// Create routing discovery
	routingDisc := routing.NewRoutingDiscovery(kademliaDHT)
	
	manager := &Manager{
		host:          h,
		dht:           kademliaDHT,
		routingDisc:   routingDisc,
		identity:      identityMgr,
		ctx:           netCtx,
		cancel:        cancel,
		peers:         make(map[peer.ID]*Peer),
		eventHandlers: make(map[string][]NetworkEventHandler),
		listenAddrs:   listenAddrs,
	}
	
	// Set up connection event handlers
	h.Network().Notify((*networkNotifiee)(manager))
	
	// Set up stream handlers
	h.SetStreamHandler(ChatProtocol, manager.handleChatStream)
	h.SetStreamHandler(TransferProtocol, manager.handleTransferStream)
	
	return manager, nil
}

// Start initializes the network manager and starts discovery
func (m *Manager) Start() error {
	log.Println("Starting network manager...")
	
	// Bootstrap DHT
	if err := m.dht.Bootstrap(m.ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}
	
	// Start mDNS discovery
	if err := m.startMDNSDiscovery(); err != nil {
		log.Printf("Failed to start mDNS discovery: %v", err)
	}
	
	// Start DHT discovery
	go m.startDHTDiscovery()
	
	// Announce ourselves
	go m.announcePresence()
	
	log.Printf("Network manager started. Listening on:")
	for _, addr := range m.host.Addrs() {
		log.Printf("  %s/p2p/%s", addr, m.host.ID().String())
	}
	
	return nil
}

// startMDNSDiscovery starts local network peer discovery using mDNS
func (m *Manager) startMDNSDiscovery() error {
	log.Printf("Starting mDNS discovery with service tag: '%s'", ServiceTag)
	log.Printf("Host ID: %s", m.host.ID().String())
	log.Printf("Host addresses:")
	for _, addr := range m.host.Addrs() {
		log.Printf("  %s", addr.String())
	}
	
	notifiee := &discoveryNotifiee{manager: m}
	service := mdns.NewMdnsService(m.host, ServiceTag, notifiee)
	if err := service.Start(); err != nil {
		log.Printf("Warning: mDNS discovery failed to start: %v", err)
		log.Printf("You may not be able to discover peers on the local network")
		return nil // Don't fail the whole app if mDNS fails
	}
	
	m.discovery = service
	log.Printf("mDNS discovery service started successfully")
	
	// Add periodic check to see if mDNS is working
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		checks := 0
		
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				checks++
				peerCount := len(m.peers)
				log.Printf("mDNS check #%d: %d peers discovered so far", checks, peerCount)
				if checks >= 4 && peerCount == 0 {
					log.Printf("Warning: No peers discovered after %d seconds. Try manual connection.", checks*15)
				}
			}
		}
	}()
	
	return nil
}

// startDHTDiscovery starts DHT-based peer discovery
func (m *Manager) startDHTDiscovery() {
	log.Println("Starting DHT discovery...")
	
	// Initial discovery attempt
	go func() {
		if _, err := m.routingDisc.Advertise(m.ctx, ServiceTag); err != nil {
			log.Printf("DHT advertising not available yet: %v (this is normal for the first instance)", err)
		} else {
			log.Printf("Successfully advertised service '%s' on DHT", ServiceTag)
		}
	}()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	discoveryCount := 0
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			discoveryCount++
			log.Printf("DHT discovery attempt #%d", discoveryCount)
			
			peerChan, err := m.routingDisc.FindPeers(m.ctx, ServiceTag)
			if err != nil {
				log.Printf("Failed to find peers via DHT: %v", err)
				continue
			}
			
			go func() {
				peersFound := 0
				for peerInfo := range peerChan {
					if peerInfo.ID == m.host.ID() {
						continue
					}
					
					peersFound++
					log.Printf("Found peer via DHT: %s", peerInfo.ID)
					
					if err := m.host.Connect(m.ctx, peerInfo); err != nil {
						log.Printf("Failed to connect to DHT peer %s: %v", peerInfo.ID, err)
					} else {
						log.Printf("Successfully connected to DHT peer: %s", peerInfo.ID)
					}
				}
				
				if peersFound == 0 {
					log.Printf("No peers found via DHT in attempt #%d", discoveryCount)
				}
			}()
		}
	}
}

// announcePresence announces our presence on the network
func (m *Manager) announcePresence() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if _, err := m.routingDisc.Advertise(m.ctx, ServiceTag); err != nil {
				// Only log this if we have peers in the DHT table
				if m.dht.RoutingTable().Size() > 0 {
					log.Printf("Failed to advertise presence: %v", err)
				}
			}
		}
	}
}

// GetPeers returns a list of connected peers
func (m *Manager) GetPeers() []*Peer {
	m.peersMutex.RLock()
	defer m.peersMutex.RUnlock()
	
	peers := make([]*Peer, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	
	return peers
}

// GetPeerCount returns the number of connected peers
func (m *Manager) GetPeerCount() int {
	m.peersMutex.RLock()
	defer m.peersMutex.RUnlock()
	return len(m.peers)
}

// GetHost returns the libp2p host
func (m *Manager) GetHost() host.Host {
	return m.host
}

// GetDHT returns the DHT instance
func (m *Manager) GetDHT() *dht.IpfsDHT {
	return m.dht
}

// SendMessage sends a message to a peer using the specified protocol
func (m *Manager) SendMessage(peerID peer.ID, protocol protocol.ID, data []byte) error {
	stream, err := m.host.NewStream(m.ctx, peerID, protocol)
	if err != nil {
		return fmt.Errorf("failed to create stream to peer %s: %w", peerID, err)
	}
	defer stream.Close()
	
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to send message to peer %s: %w", peerID, err)
	}
	
	return nil
}

// AddEventHandler adds a network event handler
func (m *Manager) AddEventHandler(name string, handler NetworkEventHandler) {
	m.handlersMutex.Lock()
	defer m.handlersMutex.Unlock()
	
	if m.eventHandlers[name] == nil {
		m.eventHandlers[name] = make([]NetworkEventHandler, 0)
	}
	m.eventHandlers[name] = append(m.eventHandlers[name], handler)
}

// RemoveEventHandler removes a network event handler
func (m *Manager) RemoveEventHandler(name string) {
	m.handlersMutex.Lock()
	defer m.handlersMutex.Unlock()
	
	delete(m.eventHandlers, name)
}

// notifyPeerConnected notifies all event handlers of a peer connection
func (m *Manager) notifyPeerConnected(peer *Peer) {
	m.handlersMutex.RLock()
	defer m.handlersMutex.RUnlock()
	
	for _, handlers := range m.eventHandlers {
		for _, handler := range handlers {
			go handler.OnPeerConnected(peer)
		}
	}
}

// notifyPeerDisconnected notifies all event handlers of a peer disconnection
func (m *Manager) notifyPeerDisconnected(peerID peer.ID) {
	m.handlersMutex.RLock()
	defer m.handlersMutex.RUnlock()
	
	for _, handlers := range m.eventHandlers {
		for _, handler := range handlers {
			go handler.OnPeerDisconnected(peerID)
		}
	}
}

// notifyMessage notifies all event handlers of a received message
func (m *Manager) notifyMessage(peerID peer.ID, protocol protocol.ID, data []byte) {
	m.handlersMutex.RLock()
	defer m.handlersMutex.RUnlock()
	
	for _, handlers := range m.eventHandlers {
		for _, handler := range handlers {
			go handler.OnMessage(peerID, protocol, data)
		}
	}
}

// handleChatStream handles incoming chat streams
func (m *Manager) handleChatStream(stream network.Stream) {
	defer stream.Close()
	
	// Read message data
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil {
		log.Printf("Failed to read chat message: %v", err)
		return
	}
	
	// Notify handlers
	m.notifyMessage(stream.Conn().RemotePeer(), ChatProtocol, buf[:n])
}

// handleTransferStream handles incoming file transfer streams
func (m *Manager) handleTransferStream(stream network.Stream) {
	defer stream.Close()
	
	// Read transfer data
	buf := make([]byte, 4096)
	n, err := stream.Read(buf)
	if err != nil {
		log.Printf("Failed to read transfer message: %v", err)
		return
	}
	
	// Notify handlers
	m.notifyMessage(stream.Conn().RemotePeer(), TransferProtocol, buf[:n])
}

// Close shuts down the network manager
func (m *Manager) Close() error {
	m.cancel()
	
	if m.discovery != nil {
		m.discovery.Close()
	}
	
	if m.dht != nil {
		m.dht.Close()
	}
	
	return m.host.Close()
}