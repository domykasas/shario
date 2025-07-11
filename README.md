# Shario - P2P File Sharing Application

Shario is a cross-platform peer-to-peer file sharing application with real-time chat capabilities. Built with Go, libp2p, and Fyne, it provides secure, decentralized file sharing without the need for central servers.

## Features

- **P2P File Sharing**: Direct file transfers between peers with real-time progress tracking
- **Real-time Chat**: Instant messaging with connected peers
- **Peer Discovery**: Automatic peer discovery using mDNS (local network) and DHT (internet)
- **Secure Communication**: End-to-end encryption using libp2p's secure transport
- **Cross-platform**: Runs on Windows, macOS, and Linux
- **User-friendly GUI**: Clean, intuitive interface built with Fyne
- **Identity Management**: Cryptographic identity with customizable nicknames

## Requirements

- Go 1.20 or higher
- libp2p dependencies (automatically handled by Go modules)
- Fyne UI dependencies

### System Dependencies

**Ubuntu/Debian:**
```bash
sudo apt-get install gcc pkg-config libgl1-mesa-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev libasound2-dev
```

**CentOS/RHEL/Fedora:**
```bash
sudo yum install gcc pkg-config mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel alsa-lib-devel
# or for newer versions:
sudo dnf install gcc pkg-config mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel alsa-lib-devel
```

**macOS:**
```bash
# Xcode command line tools are required
xcode-select --install
```

**Windows:**
- Go 1.20+ (includes CGO support)
- GCC compiler (TDM-GCC or MinGW-w64 recommended)

## Building and Running

### Clone the Repository
```bash
git clone https://github.com/yourusername/shario.git
cd shario
```

### Install Dependencies
```bash
go mod download
```

### Build the Application
```bash
go build -o shario .
```

### Run the Application
```bash
./shario
```

### Build for Different Platforms

**Build for Windows (from any platform):**
```bash
GOOS=windows GOARCH=amd64 go build -o shario.exe .
```

**Build for macOS (from any platform):**
```bash
GOOS=darwin GOARCH=amd64 go build -o shario .
```

**Build for Linux (from any platform):**
```bash
GOOS=linux GOARCH=amd64 go build -o shario .
```

## Usage

### First Launch
1. Run the application: `go run .`
2. Set your nickname in the "Your Identity" section (type new name and click "Update")  
3. The application will automatically start discovering peers on your local network
4. To test with multiple instances, open additional terminals and run `go run .`

### Connecting to Peers
- Peers on the same local network will be discovered automatically via mDNS
- For internet-wide discovery, peers connect through the DHT network
- Connected peers will appear in the "Peers" tab

### Sending Files
1. Go to the "Peers" tab
2. Click "Send File" next to a connected peer
3. Select the file you want to send
4. The transfer will appear in the "Transfers" tab with real-time progress
5. Use "Cancel" button to stop transfers, "Open" button to view completed files

### Receiving Files
- When someone sends you a file, you'll see a dialog asking if you want to accept it
- Click "Yes" to accept the transfer
- Accepted files are saved to your Downloads/Shario folder
- Transfer progress is shown in the "Transfers" tab with real-time updates
- Use "Open" button to open received files or their containing folder

### Chatting
1. **Global Chat**: Automatically available when you start Shario
   - All connected users join the global chat automatically
   - Start typing immediately - no setup required
   - Messages are visible to all connected peers
2. **Direct Chat**: Click "Chat" next to a specific peer for private messages
3. Type messages and press Enter or click "Send"
4. Chat history is maintained for the session

## Configuration

### Identity Storage
Your cryptographic identity is stored in:
- **Linux/macOS**: `~/.shario/identity_[PID].json` (unique per instance)
- **Windows**: `%USERPROFILE%\.shario\identity_[PID].json`

Note: Each running instance creates a unique identity file based on its process ID, allowing multiple instances to run simultaneously for testing.

### Download Directory
Files are downloaded to:
- **Linux/macOS**: `~/Downloads/Shario/`
- **Windows**: `%USERPROFILE%\Downloads\Shario\`

## Architecture

The application follows a modular architecture:

- **`main.go`**: Application entry point
- **`internal/app/`**: Main application controller
- **`internal/network/`**: P2P networking using libp2p
- **`internal/transfer/`**: File transfer management
- **`internal/chat/`**: Real-time chat functionality
- **`internal/identity/`**: Identity and key management
- **`internal/ui/`**: User interface using Fyne

## Security

- All peer communications use libp2p's secure transport layer
- Each user has a unique cryptographic identity based on RSA key pairs
- File transfers are encrypted end-to-end
- No central server required - fully decentralized

## Troubleshooting

### Common Issues

**Application won't start:**
- Ensure all system dependencies are installed
- Check that Go version is 1.20 or higher
- Verify CGO is enabled: `go env CGO_ENABLED`

**No peers discovered:**
- **Most Common**: You need to run multiple Shario instances to see peers
  - Open 2+ terminals and run `go run .` in each
  - Each instance creates a unique identity automatically
  - Peers should discover each other within 10-15 seconds
- **Network Issues**: 
  - Check firewall settings - ensure the application can accept incoming connections
  - Verify you're on the same network as other Shario users (for mDNS discovery)
  - Wait 30-60 seconds for initial DHT discovery
- **Manual Connection**: Use "Connect to Peer" button with peer's multiaddress

**File transfers failing:**
- Ensure both peers have stable network connections
- Check available disk space
- Verify firewall allows peer-to-peer connections

**GUI not displaying correctly:**
- Update graphics drivers
- Try running with software rendering: `FYNE_THEME=dark ./shario`

### Debug Mode
Run with debug logging:
```bash
export LIBP2P_DEBUG=1
./shario
```

## Development

### Running Tests
```bash
go test ./...
```

### Code Structure
```
shario/
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── internal/
│   ├── app/                # Main application logic
│   ├── network/            # P2P networking
│   ├── transfer/           # File transfer system
│   ├── chat/               # Chat functionality
│   ├── identity/           # Identity management
│   └── ui/                 # User interface
└── README.md               # This file
```

### Contributing
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under the MIT License. See the LICENSE file for details.

## Acknowledgments

- Built with [libp2p](https://github.com/libp2p/go-libp2p) for P2P networking
- UI built with [Fyne](https://fyne.io/) for cross-platform GUI
- Inspired by the need for decentralized, secure file sharing

## Current Status

**✅ FULLY FUNCTIONAL** - Version 1.0.2 with comprehensive package ecosystem:
- ✅ P2P file transfers with chunked streaming and progress tracking
- ✅ Real-time chat with nickname synchronization across peers
- ✅ Automatic peer discovery (mDNS local + DHT internet-wide)
- ✅ Cross-platform file operations (open/cancel transfers)
- ✅ Unique identity system preventing self-connection issues
- ✅ Compact, responsive UI with full peer ID display
- ✅ Optimized GitHub Actions workflows based on Tala's proven approach
- ✅ Multi-platform support (Linux, Windows, macOS, FreeBSD) with ARM64 architectures
- ✅ **Complete Linux package ecosystem**: DEB, RPM, Snap, AppImage, and binary formats
- ✅ **Professional packaging**: Desktop integration, dependencies, and system compatibility

## Roadmap

- [ ] Group chat rooms
- [ ] File sharing with multiple peers simultaneously  
- [ ] Mobile app support
- [ ] Advanced peer discovery methods
- [ ] File synchronization
- [ ] Voice/video chat integration
- [ ] Plugin system for extensions