// Package identity manages user identity and cryptographic keys
package identity

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Identity represents a user's identity
type Identity struct {
	Nickname   string `json:"nickname"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	PeerID     string `json:"peer_id"`
}

// Manager handles identity management
type Manager struct {
	identity   *Identity
	privateKey crypto.PrivKey
	publicKey  crypto.PubKey
	peerID     peer.ID
	configPath string
}

// New creates a new identity manager
func New() (*Manager, error) {
	// Get config directory
	configDir, err := getConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}
	
	// Create unique identity file for each instance
	// This allows multiple instances to run with different identities
	configPath := filepath.Join(configDir, fmt.Sprintf("identity_%d.json", os.Getpid()))
	
	manager := &Manager{
		configPath: configPath,
	}
	
	// Load existing identity or create new one
	if err := manager.loadOrCreateIdentity(); err != nil {
		return nil, fmt.Errorf("failed to load or create identity: %w", err)
	}
	
	return manager, nil
}

// loadOrCreateIdentity loads existing identity or creates a new one
func (m *Manager) loadOrCreateIdentity() error {
	// Try to load existing identity
	if _, err := os.Stat(m.configPath); err == nil {
		return m.loadIdentity()
	}
	
	// Create new identity
	return m.createIdentity()
}

// loadIdentity loads identity from file
func (m *Manager) loadIdentity() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read identity file: %w", err)
	}
	
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return fmt.Errorf("failed to unmarshal identity: %w", err)
	}
	
	// Parse private key
	privateKeyBytes, err := crypto.ConfigDecodeKey(identity.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}
	
	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	
	// Get public key
	publicKey := privateKey.GetPublic()
	
	// Generate peer ID
	peerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to generate peer ID: %w", err)
	}
	
	m.identity = &identity
	m.privateKey = privateKey
	m.publicKey = publicKey
	m.peerID = peerID
	
	return nil
}

// createIdentity creates a new identity
func (m *Manager) createIdentity() error {
	// Generate keypair
	privateKey, publicKey, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}
	
	// Generate peer ID
	peerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to generate peer ID: %w", err)
	}
	
	// Marshal keys
	privateKeyBytes, err := crypto.MarshalPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	
	publicKeyBytes, err := crypto.MarshalPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	
	// Create identity
	identity := &Identity{
		Nickname:   "Anonymous",
		PublicKey:  crypto.ConfigEncodeKey(publicKeyBytes),
		PrivateKey: crypto.ConfigEncodeKey(privateKeyBytes),
		PeerID:     peerID.String(),
	}
	
	// Save identity
	if err := m.saveIdentity(identity); err != nil {
		return fmt.Errorf("failed to save identity: %w", err)
	}
	
	m.identity = identity
	m.privateKey = privateKey
	m.publicKey = publicKey
	m.peerID = peerID
	
	return nil
}

// saveIdentity saves identity to file
func (m *Manager) saveIdentity(identity *Identity) error {
	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}
	
	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}
	
	return nil
}

// GetNickname returns the user's nickname
func (m *Manager) GetNickname() string {
	return m.identity.Nickname
}

// SetNickname sets the user's nickname
func (m *Manager) SetNickname(nickname string) error {
	m.identity.Nickname = nickname
	return m.saveIdentity(m.identity)
}

// GetPrivateKey returns the user's private key
func (m *Manager) GetPrivateKey() crypto.PrivKey {
	return m.privateKey
}

// GetPublicKey returns the user's public key
func (m *Manager) GetPublicKey() crypto.PubKey {
	return m.publicKey
}

// GetPeerID returns the user's peer ID
func (m *Manager) GetPeerID() peer.ID {
	return m.peerID
}

// GetIdentity returns the user's identity
func (m *Manager) GetIdentity() *Identity {
	return m.identity
}

// VerifyIdentity verifies a peer's identity using their public key
func (m *Manager) VerifyIdentity(peerID peer.ID, publicKey crypto.PubKey) error {
	// Generate peer ID from public key
	expectedPeerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to generate peer ID from public key: %w", err)
	}
	
	// Compare peer IDs
	if peerID != expectedPeerID {
		return fmt.Errorf("peer ID mismatch: expected %s, got %s", expectedPeerID, peerID)
	}
	
	return nil
}

// SignData signs data with the user's private key
func (m *Manager) SignData(data []byte) ([]byte, error) {
	return m.privateKey.Sign(data)
}

// VerifySignature verifies a signature using a public key
func (m *Manager) VerifySignature(data, signature []byte, publicKey crypto.PubKey) (bool, error) {
	return publicKey.Verify(data, signature)
}

// getConfigDir returns the configuration directory
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	configDir := filepath.Join(homeDir, ".shario")
	return configDir, nil
}

// ExportIdentity exports the identity for backup
func (m *Manager) ExportIdentity() ([]byte, error) {
	return json.MarshalIndent(m.identity, "", "  ")
}

// ImportIdentity imports an identity from backup
func (m *Manager) ImportIdentity(data []byte) error {
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return fmt.Errorf("failed to unmarshal identity: %w", err)
	}
	
	// Validate the identity
	privateKeyBytes, err := crypto.ConfigDecodeKey(identity.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}
	
	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal private key: %w", err)
	}
	
	publicKey := privateKey.GetPublic()
	peerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to generate peer ID: %w", err)
	}
	
	if peerID.String() != identity.PeerID {
		return fmt.Errorf("peer ID mismatch in imported identity")
	}
	
	// Save the new identity
	if err := m.saveIdentity(&identity); err != nil {
		return fmt.Errorf("failed to save imported identity: %w", err)
	}
	
	// Update manager state
	m.identity = &identity
	m.privateKey = privateKey
	m.publicKey = publicKey
	m.peerID = peerID
	
	return nil
}