# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
**Shario** - Cross-platform P2P file-sharing application with real-time chat
- **Current Version**: 1.0.6 (Stable Release - 2025-07-11)
- **Language**: Go 1.20+
- **GUI**: Fyne framework
- **Networking**: libp2p (mDNS + DHT discovery)
- **Architecture**: Modular packages (app, network, transfer, chat, identity, ui)
- **Status**: ✅ FULLY FUNCTIONAL - All core features working

## Essential Commands

### Development
```bash
# Run GUI version
go run .

# Run headless version (for cross-compilation testing)
go run -tags headless .

# Build GUI version
go build -o shario .

# Build headless version
go build -tags headless -o shario-headless .

# Run tests
go test ./...

# Debug with filtered output
go run . 2>&1 | grep -E "(🎭|📁|🎯|🗂️|📥|📤)"
```

### Multi-Instance Testing
```bash
# Open multiple terminals and run:
go run .
# Each instance creates a unique identity based on PID
# Peers discover each other automatically within 10-15 seconds
```

## Core Architecture

### Application Structure
```
internal/
├── app/           # Main application controller
│   ├── app.go     # GUI version with Fyne
│   └── app_headless.go # Headless version for ARM64/FreeBSD
├── identity/      # RSA key management & peer identity
├── network/       # libp2p networking (mDNS + DHT)
├── transfer/      # Chunked file transfer system
├── chat/          # Real-time messaging
└── ui/            # Fyne GUI components
```

### Key Design Patterns

**Event-Driven Architecture:**
```go
// Network events flow through managers
networkMgr.AddEventHandler("chat", chatMgr)
networkMgr.AddEventHandler("transfer", transferMgr)
```

**Build Tag Separation:**
- `main.go` + `app.go`: GUI builds (default)
- `main_headless.go` + `app_headless.go`: Headless builds (`-tags headless`)

**Identity System:**
- Each instance: `~/.shario/identity_[PID].json`
- Prevents self-connection in multi-instance testing
- RSA keys for P2P authentication

**File Transfer Protocol:**
- 1KB chunks, base64 encoded for JSON transport (reduced from 4KB to avoid network message size limits)
- Flow: Offer → Accept → Data chunks → Complete
- Storage: `~/Downloads/Shario/`

## GitHub Actions Release Configuration

### Asset Naming Convention
Following **Tala's format exactly**: `shario-v{version}-{platform}-{arch}[-headless]`

**Critical Rules:**
1. **ALWAYS include "headless" suffix** for builds without GUI (ARM64, FreeBSD)
2. **Use "x64" not "amd64"** for Windows and archive formats
3. **Use "amd64" for Linux** binary and package formats  
4. **Version format**: `v{MAJOR}.{MINOR}.{PATCH}-rc.{N}` (with 'v' prefix)

### Build Matrix
```yaml
# GUI builds (native compilation with CGO/Fyne)
- Linux x64: ubuntu-latest, CGO enabled, create_packages: true
- Windows x64: windows-latest, CGO enabled, create_installer: true
- macOS Intel: macos-13, CGO enabled, create_dmg: true
- macOS ARM64: macos-latest, CGO enabled, create_dmg: true

# Headless builds (cross-compilation, CGO disabled)
- Linux ARM64: ubuntu-latest, cross_compile: true, headless_mode: true
- Windows ARM64: windows-latest, cross_compile: true, headless_mode: true
- FreeBSD x64: ubuntu-latest, cross_compile: true, headless_mode: true
```

### Generated Package Formats
**Linux (GUI x64):**
- Binary: `shario-v{version}-linux-amd64`
- DEB: `shario-v{version}-linux-amd64.deb`
- RPM: `shario-v{version}-linux-x86_64.rpm`
- Snap: `shario-v{version}-linux-amd64.snap`
- AppImage: `shario-v{version}-linux-x86_64.AppImage`
- Archive: `shario-v{version}-linux-x64.tar.xz`

**Windows (GUI x64):**
- Executable: `shario-v{version}-windows-x64.exe`
- Squirrel: `shario-v{version}-squirrel.zip`

**macOS (GUI Intel/ARM64):**
- DMG: `shario-v{version}-macos.dmg`
- Binaries: `shario-v{version}-macos-amd64`, `shario-v{version}-macos-arm64`

**Headless builds:**
- Linux ARM64: `shario-v{version}-linux-arm64-headless`
- Windows ARM64: `shario-v{version}-windows-arm64-headless.exe`
- FreeBSD x64: `shario-v{version}-freebsd-x64-headless`

## Common Development Issues

### File transfer empty files
JSON message size limits cause "unexpected end of JSON input" errors
- Fix: Reduce chunk size from 4KB to 1KB to avoid network message limits
- Base64 encoding + JSON overhead can exceed protocol limits

### Missing headless indicators
ARM64 and FreeBSD builds MUST include "headless" in filename

### RPM build errors
Version mismatch between tarball directory and RPM spec
- Fix: Use `$RPM_VERSION` consistently

### Windows build timeouts
- Fix: Use build caching, parallel compilation, 30min timeout

### Cross-compilation issues
CGO conflicts with GUI frameworks
- Fix: Use `headless_mode: true` with `CGO_ENABLED=0`

## Release Management

### Version Format
- **Pre-release**: `v1.0.0-rc.N` (testing versions)
- **Stable**: `v1.0.0` (production releases)
- **Semver**: MAJOR.MINOR.PATCH (breaking.feature.bugfix)

### Release Process
1. **Update version** in CLAUDE.md
2. **Update CHANGELOG.md** (move [Unreleased] to version section)
3. **Update README.md** (if version referenced)
4. **Commit & tag**: `git tag v1.0.0-rc.N`
5. **Push when requested**: Human controls all git push operations

### Critical Workflow Reminders
- **NEVER add Claude Code attribution** to git commits
- **ALWAYS update CHANGELOG.md** after any file operation
- **Auto-increment RC version** before each release push
- **Human controls git push** - wait for explicit request

## Known Working Features
✅ **Peer Discovery**: Automatic mDNS (local) + DHT (internet) discovery
✅ **Real-time Chat**: Global chat room with nickname synchronization
✅ **File Transfers**: Chunked transfer system with progress tracking
✅ **Cross-platform**: Linux/macOS/Windows/FreeBSD support
✅ **Identity System**: Unique per-instance identities prevent self-connection
✅ **Network Stability**: Multi-connection handling, proper disconnection detection
✅ **Professional Packaging**: Complete ecosystem matching Tala's format

## Development Notes
- Use unique identity files per process (`identity_[PID].json`) for multi-instance testing
- File transfers use 1KB chunks to avoid network message size limits (base64 + JSON overhead)
- Chat messages use dynamic nickname lookup from current peer state
- UI uses Fyne data binding for real-time updates
- Cross-platform file operations detect runtime.GOOS
- Headless builds enable ARM64/FreeBSD support without GUI dependencies

## Claude Code Working Style & Recent Successful Patterns

### Problem-Solving Approach
**When user reports issues:**
1. **Understand the problem** - Read error messages carefully, identify root cause
2. **Fix systematically** - Make targeted changes, don't over-engineer
3. **Update documentation** - Always update CHANGELOG.md and version references
4. **Test thoroughly** - Consider edge cases and platform differences

### Recent Successful Fixes
**File Transfer Bug (Empty Files):**
- **Root Cause**: 4KB chunks + base64 encoding exceeded network message limits
- **Solution**: Reduced chunk size from 4KB to 1KB in `transfer/manager.go:604`
- **Key Learning**: JSON + base64 overhead can exceed protocol limits even with small chunks

**GitHub Actions Workflow Issues:**
- **DMG Artifact Conflict**: Both macOS builds used same artifact name
- **Solution**: Made architecture-specific: `shario-macos-dmg-{arch}-VERSION`
- **Windows Zip Error**: `zip` command not available on Windows runners
- **Solution**: Used PowerShell `Compress-Archive` for Windows builds

### Effective Communication Pattern
- **Be concise** - Answer directly without unnecessary explanation
- **Use specific file paths** - Reference exact locations like `transfer/manager.go:604`
- **Show what changed** - Explain the fix but don't over-explain
- **Proactive versioning** - Auto-increment versions for each significant fix

### Version Management Strategy
- **Patch releases** (1.0.1 → 1.0.2 → 1.0.3) for bug fixes
- **Update all references** in CLAUDE.md, README.md, CHANGELOG.md
- **Clear changelog entries** with specific technical details
- **Git workflow**: add → commit → tag → push (when requested)

### Key Technical Details to Remember
**File Transfer System:**
- Location: `internal/transfer/manager.go`
- Chunk size: 1KB (line 604: `const chunkSize = 1024`)
- Base64 encoding for JSON transport
- Flow: Offer → Accept → Data chunks → Complete

**GitHub Actions:**
- Windows builds use PowerShell `Compress-Archive` not `zip`
- macOS DMG artifacts need architecture-specific naming
- Headless builds use `CGO_ENABLED=0` with `-tags headless`

**Build Matrix:**
- 7 platform/architecture combinations
- GUI builds: Linux x64, Windows x64, macOS Intel/ARM64
- Headless builds: Linux ARM64, Windows ARM64, FreeBSD x64

### User Interaction Style
- **Direct requests** like "Do git push" - execute immediately without questions
- **Problem reports** - Analyze, fix, update docs, version bump
- **Collaborative** - User provides context, I provide technical solutions
- **Efficient** - Minimize back-and-forth, be proactive with related fixes

### Quality Standards
- **All changes** must update CHANGELOG.md
- **ALWAYS bump version** for ANY changes (even documentation improvements)
- **Version consistency** across all documentation files (CLAUDE.md, README.md, CHANGELOG.md)
- **Professional git commits** with clear, technical messages
- **No Claude attribution** in git commits (user preference)
- **Human controls git push** - wait for explicit request

### Critical Version Management Rule
🚨 **NEVER FORGET**: Every commit must include a version bump, no exceptions:
- Documentation changes: PATCH version (1.0.3 → 1.0.4)
- Bug fixes: PATCH version (1.0.3 → 1.0.4)
- New features: MINOR version (1.0.3 → 1.1.0)
- Breaking changes: MAJOR version (1.0.3 → 2.0.0)

**Files to update for every version bump:**
1. `CLAUDE.md` - Current Version line
2. `README.md` - Version badge AND Current Status section
3. `CHANGELOG.md` - Add new version section with changes
4. Git tag: `git tag v1.0.X`

### Critical Build Testing Rule
🚨 **NEVER USE "go build"** - Use "go test" instead:
- **Don't use**: `go build .` (creates executable in directory)
- **Use instead**: `go test ./...` (tests compilation without creating files)
- **Reason**: Executables are large (50+ MB) and bloat git repository
- **Human preference**: User will handle builds themselves when needed
- **Testing compilation**: `go test ./...` verifies code compiles correctly
- **Only exception**: When explicitly asked to build by user