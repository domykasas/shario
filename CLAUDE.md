# CLAUDE.md - Project Memory

## Project Overview
**Shario** - Cross-platform P2P file-sharing application with real-time chat
- **Current Version**: 0.0.1 (Initial Release - 2025-07-10)
- **Language**: Go 1.20+
- **GUI**: Fyne framework  
- **Networking**: libp2p (mDNS + DHT discovery)
- **Architecture**: Modular packages (app, network, transfer, chat, identity, ui)
- **Status**: ✅ FULLY FUNCTIONAL - All core features working

## Versioning
- **Follows**: [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- **Changelog**: [Keep a Changelog](https://keepachangelog.com/) format
- **Current**: v0.0.1 - Initial release with all core P2P functionality
- **Next**: v0.1.0 - First minor release (when adding new features)

## Key Technical Details

### **Identity System**
- Each instance uses unique identity file: `~/.shario/identity_[PID].json`
- RSA keys + peer IDs for P2P authentication
- Nickname management with real-time broadcasting

### **Network Architecture**
- **Protocols**: `/shario/chat/1.0.0`, `/shario/transfer/1.0.0`
- **Discovery**: mDNS (local) + DHT (internet-wide)
- **Connection stability**: Handles multiple connections per peer (IPv4/IPv6)

### **File Transfer System**
- **Chunking**: 4KB chunks, base64 encoded for JSON transport
- **Flow**: Offer → Accept → Data chunks → Complete
- **Storage**: `~/Downloads/Shario/`
- **Progress tracking**: Real-time updates in UI

### **Chat System**
- **Global room**: Auto-connects all discovered peers
- **Nickname sync**: Broadcasts changes to all connected peers
- **Message format**: `[HH:MM:SS] Nickname: Message` (compact 1-line)

## Recent Bug Fixes

### **Nickname Changes** ✅
- **Issue**: UI field didn't update, changes weren't broadcasted
- **Fix**: Added UI field update + focus handling with Update button
- **Location**: `internal/ui/manager.go:155-195`

### **File Transfer Dialog** ✅
- **Issue**: Always returned `false` (async/sync mismatch)
- **Fix**: Used Go channel to wait for user response
- **Location**: `internal/ui/manager.go:815-833`

### **File Content Transfer** ✅
- **Issue**: Files created but empty (no actual content transfer)
- **Fix**: Implemented chunked file streaming with base64 encoding
- **Location**: `internal/transfer/manager.go:551-728`

### **Transfer Buttons** ✅
- **Cancel**: Calls `CancelTransfer()` with error handling
- **Open**: Cross-platform file/folder opening (xdg-open/open/explorer)
- **Location**: `internal/ui/manager.go:331-878`

## UI Improvements Made

### **Layout Fixes**
- Larger nickname input field with Update button (border layout)
- Compact chat messages (1 line instead of 3)
- Full peer IDs displayed (no truncation with "...")

### **Debugging Added**
- 🎭 Nickname changes
- 📁 File transfers 
- 🎯 Dialog interactions
- 🗂️ File operations
- 📥📤 Network messages

## Architecture Patterns

### **Event Handling**
```go
// Network events → Chat/Transfer managers → UI updates
networkMgr.AddEventHandler("chat", chatMgr)
networkMgr.AddEventHandler("transfer", transferMgr)
```

### **Message Protocols**
```go
// Chat: text, system, join, leave, nickname_change
// Transfer: offer, accept, reject, data, complete, cancel
```

### **UI Data Binding**
```go
// Fyne data binding for real-time UI updates
peersData, transfersData, messagesData binding.StringList
```

## Common Commands
- **Test**: `go test ./...`
- **Run**: `go run .`
- **Debug**: `go run . 2>&1 | grep -E "(🎭|📁|🎯|🗂️|📥|📤)"`

## 🚨 CRITICAL WORKFLOW REMINDERS

### 📝 CHANGELOG.md Updates
**ALWAYS update CHANGELOG.md after ANY file operation:**
- **Create file**: Add "Added [filename] - [purpose]" to [Unreleased]
- **Delete file**: Add "Removed [filename] - [reason]" to [Unreleased]  
- **Update file**: Add "Changed [filename] - [what changed]" to [Unreleased]
- **Bug fix**: Add "Fixed [issue description]" to [Unreleased]
- **New feature**: Add "Added [feature description]" to [Unreleased]

### 🚫 GIT COMMIT ATTRIBUTION
**NEVER add Claude Code attribution to git commits:**
- ❌ Do NOT add: "🤖 Generated with [Claude Code](https://claude.ai/code)"
- ❌ Do NOT add: "Co-Authored-By: Claude <noreply@anthropic.com>"
- ✅ Keep commits clean and professional without AI attribution
- ✅ Let the human developer take full credit for their work

**Example entries**:
```
## [Unreleased]
### Added
- Added CHANGELOG.md following Keep a Changelog standards
- Added version management documentation to CLAUDE.md
### Changed
- Updated README.md with current project status
### Fixed
- Fixed file transfer dialog async/sync mismatch
```

## Known Working Features
✅ **Peer Discovery**: Automatic mDNS (local) + DHT (internet) discovery
✅ **Real-time Chat**: Global chat room with instant nickname synchronization  
✅ **File Transfers**: Complete chunked transfer system with progress tracking
✅ **Cross-platform**: Linux/macOS/Windows support with native file operations
✅ **Transfer Management**: Working Cancel and Open buttons with error handling
✅ **UI/UX**: Compact layout, full peer IDs, responsive nickname updates
✅ **Identity**: Unique per-instance identity files prevent self-connection issues
✅ **Network Stability**: Multi-connection handling, proper disconnection detection

## Version Management
- **CHANGELOG.md**: Update for all releases following keepachangelog.com format
- **Version Ordering**: NEWEST versions go at TOP of CHANGELOG.md (newest to oldest)
  - Example: v0.0.2 (top) → v0.0.1 (below) → older versions (bottom)
  - New releases are added ABOVE existing entries, after [Unreleased] section
  - **Changelog Structure Example**:
    ```
    # Changelog
    ## [Unreleased]
    ## [0.0.2] - 2025-07-11  ← NEWEST (top)
    ### Added
    - New feature...
    ## [0.0.1] - 2025-07-10  ← OLDER (below)
    ### Added
    - Initial release...
    ```
- **Version Bumping Rules**:
  - PATCH (0.0.X): Bug fixes, no new features
  - MINOR (0.X.0): New features, backwards compatible
  - MAJOR (X.0.0): Breaking changes, incompatible API changes
- **Release Process**: Update CHANGELOG.md → Update version references → Tag release
- **Files to Update**: CHANGELOG.md, CLAUDE.md, README.md (if version mentioned)
- **⚠️ IMPORTANT REMINDER**: After ANY file Create/Delete/Update operation, ALWAYS update CHANGELOG.md
  - Add entry to [Unreleased] section immediately after making changes
  - Document what was changed, added, or removed
  - Keep changelog current throughout development, not just at release time

## Development Notes
- Use unique identity files per process for multi-instance testing
- File transfers use 4KB chunks to avoid network message size limits
- Chat messages use current peer nicknames (dynamic lookup)
- UI uses border layouts for better space allocation
- Cross-platform file operations via runtime.GOOS detection