# CLAUDE.md - Project Memory

## Project Overview
**Shario** - Cross-platform P2P file-sharing application with real-time chat
- **Current Version**: 0.0.1-rc.9 (Release Candidate - 2025-07-10)
- **Language**: Go 1.20+
- **GUI**: Fyne framework  
- **Networking**: libp2p (mDNS + DHT discovery)
- **Architecture**: Modular packages (app, network, transfer, chat, identity, ui)
- **Status**: ✅ FULLY FUNCTIONAL - All core features working

## Versioning
- **Follows**: [Semantic Versioning](https://semver.org/) (MAJOR.MINOR.PATCH)
- **Changelog**: [Keep a Changelog](https://keepachangelog.com/) format
- **Current**: v0.0.1-rc.9 - Release candidate with all core P2P functionality + workflow fixes + Go formatting compliance
- **Next**: v0.0.1 - Stable release (after testing rc.9)

### Pre-release Versions (Release Candidates)
- **Format**: `MAJOR.MINOR.PATCH-rc.N` (e.g., 0.0.1-rc.1, 0.0.1-rc.2)
- **Purpose**: Testing versions before stable release
- **Workflow**:
  1. `v0.0.1-rc.1` - First release candidate
  2. `v0.0.1-rc.2` - Second release candidate (if bugs found in rc.1)
  3. `v0.0.1-rc.N` - Additional candidates as needed
  4. `v0.0.1` - Final stable release (when rc.N is bug-free)
- **GitHub Actions**: Release candidates trigger same CI/CD workflows as stable releases
- **Distribution**: Pre-releases are marked as "Pre-release" on GitHub releases page
- **Testing**: Use rc versions for user acceptance testing before stable release
- **Git Tags**: Each rc gets its own tag (`git tag v0.0.1-rc.1`, `git push origin v0.0.1-rc.1`)
- **Changelog**: Document rc releases in CHANGELOG.md under [Unreleased] until stable release

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

### 📖 README.md Updates
**ALWAYS check and update README.md after ANY file operation:**
- **Create new feature/module**: Update Features section, Usage instructions, Architecture diagram
- **Delete functionality**: Remove from Features section, update Usage instructions
- **Update core functionality**: Review and update relevant sections (Features, Usage, Requirements, etc.)
- **Add new dependencies**: Update Requirements section, Installation instructions
- **Change file structure**: Update Architecture section, Code Structure diagram
- **Fix major bugs**: Update Current Status section, remove from Known Issues if applicable
- **Add new commands/workflows**: Update Usage section, Development section
- **Version changes**: Update version references, download links, installation instructions

### 🚫 GIT COMMIT ATTRIBUTION
**NEVER add Claude Code attribution to git commits:**
- ❌ Do NOT add: "🤖 Generated with [Claude Code](https://claude.ai/code)"
- ❌ Do NOT add: "Co-Authored-By: Claude <noreply@anthropic.com>"
- ✅ Keep commits clean and professional without AI attribution
- ✅ Let the human developer take full credit for their work

### 🔒 GIT PUSH POLICY
**ONLY the human developer controls git push operations:**
- ❌ Do NOT perform "git push" automatically or without explicit request
- ✅ Can suggest "git push" when appropriate (after commits, for releases, etc.)
- ✅ Always wait for human confirmation before executing git push commands
- ✅ Let the human decide when and what to push to remote repository
- ✅ Only perform git add, git commit, and git tag operations when explicitly requested
- ✅ Human maintains full control over what gets published to remote repository

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
  - **Pre-release (rc.N)**: Testing versions before stable release
- **Release Process**: 
  - **For Release Candidates**: Update CHANGELOG.md → Update version references → Tag release (`v0.0.1-rc.1`)
  - **For Stable Release**: Update CHANGELOG.md (move from [Unreleased] to version) → Tag release (`v0.0.1`)
- **Files to Update**: CHANGELOG.md, CLAUDE.md, README.md (if version mentioned)
- **Pre-release Workflow**:
  1. Development → Create `v0.0.1-rc.1` → Test → Fix bugs → Create `v0.0.1-rc.2` → Repeat until stable
  2. When rc.N is bug-free → Create final `v0.0.1` stable release
  3. Move changelog entries from [Unreleased] to [0.0.1] section for stable release
- **⚠️ IMPORTANT REMINDER**: After ANY file Create/Delete/Update operation, ALWAYS update BOTH:
  1. **CHANGELOG.md**: Add entry to [Unreleased] section immediately after making changes
  2. **README.md**: Review and update relevant sections to reflect current project state
  - Document what was changed, added, or removed in both files
  - Keep documentation current throughout development, not just at release time
  - Ensure README.md accurately represents current features, usage, and project status

### 🔄 VERSION RELEASE WORKFLOW
**When creating a new version release (rc.X or stable), ALWAYS update these files:**
1. **CLAUDE.md**: Update "Current Version" and version references throughout
2. **README.md**: Update version numbers, download links, installation instructions
3. **CHANGELOG.md**: Move [Unreleased] items to new version section with release date
4. **Git operations**: Add, commit, tag, and push (when explicitly requested)
- **Pre-release workflow**: Development → Update docs → Commit → Tag `v0.0.1-rc.N` → Push
- **Stable release workflow**: Final testing → Update docs → Commit → Tag `v0.0.1` → Push
- **Remember**: Each release must have consistent version numbers across all documentation

### 🔢 PRE-RELEASE VERSION INCREMENT RULE
**BEFORE EVERY "git push" with release candidate versions:**
1. **Auto-increment RC version**: Current version + 1
   - Example: `0.0.1-rc.2` → `0.0.1-rc.3`
   - Example: `0.0.1-rc.5` → `0.0.1-rc.6`
   - Example: `0.1.0-rc.1` → `0.1.0-rc.2`
2. **Update sequence BEFORE git push**:
   - Step 1: Increment version number in CLAUDE.md
   - Step 2: Update CHANGELOG.md (move [Unreleased] to new rc.X section)
   - Step 3: Update README.md (if version references exist)
   - Step 4: Git add, commit, tag with new version
   - Step 5: Git push (when explicitly requested)
3. **Version format**: Always use `v0.0.1-rc.N` for tags (with 'v' prefix)
4. **Exception**: Only skip increment when moving from rc.N to stable (e.g., rc.3 → 0.0.1)

## Development Notes
- Use unique identity files per process for multi-instance testing
- File transfers use 4KB chunks to avoid network message size limits
- Chat messages use current peer nicknames (dynamic lookup)
- UI uses border layouts for better space allocation
- Cross-platform file operations via runtime.GOOS detection