#!/bin/bash

# AgentSmith-HUB Script Generator
# This script creates deployment scripts for the AgentSmith-HUB package

set -e

# Check if target directory is provided
if [ $# -lt 1 ] || [ $# -gt 2 ]; then
    echo "Usage: $0 <target_directory> [architecture]"
    echo "This script creates start.sh and stop.sh in the target directory"
    echo "Architecture: amd64 (default) or arm64"
    exit 1
fi

TARGET_DIR="$1"
ARCHITECTURE="${2:-amd64}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Validate architecture
if [ "$ARCHITECTURE" != "amd64" ] && [ "$ARCHITECTURE" != "arm64" ]; then
    echo "Error: Architecture must be 'amd64' or 'arm64'"
    exit 1
fi

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Ensure target directory exists
mkdir -p "$TARGET_DIR"

print_info "Creating deployment scripts in $TARGET_DIR"
print_info "Target architecture: $ARCHITECTURE"

# Create start.sh (based on run.sh)
if [ -f "$SCRIPT_DIR/run.sh" ]; then
    print_info "Creating start.sh..."
    cp "$SCRIPT_DIR/run.sh" "$TARGET_DIR/start.sh"
    chmod +x "$TARGET_DIR/start.sh"
else
    print_warn "Source run.sh not found, creating basic start.sh..."
    cat > "$TARGET_DIR/start.sh" << 'EOF'
#!/bin/bash

# AgentSmith-HUB Start Script

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Configuration
BINARY_NAME="agentsmith-hub"
CONFIG_ROOT="config"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if binary exists
if [ ! -f "$BINARY_NAME" ]; then
    print_error "Binary $BINARY_NAME not found!"
    exit 1
fi

# Set library path if lib directory exists
if [ -d "lib" ]; then
    export LD_LIBRARY_PATH="$(pwd)/lib:${LD_LIBRARY_PATH}"
    print_info "Set library path: $(pwd)/lib"
fi

# Check config directory
if [ ! -d "$CONFIG_ROOT" ]; then
    print_error "Config directory $CONFIG_ROOT not found!"
    exit 1
fi

# Make binary executable
chmod +x "$BINARY_NAME"

print_info "Starting AgentSmith-HUB..."
print_info "Web interface will be available at: http://localhost:8080"
print_info "Press Ctrl+C to stop"

# Start the application
if [ "$1" = "--follower" ]; then
    print_info "Starting AgentSmith-HUB in follower mode..."
    if [ -n "$LEADER_ADDR" ]; then
        exec "./$BINARY_NAME" -config_root "$CONFIG_ROOT" --follower "$LEADER_ADDR"
    else
        print_error "LEADER_ADDR environment variable is required for follower mode"
        exit 1
    fi
else
    print_info "Starting AgentSmith-HUB in leader mode..."
    exec "./$BINARY_NAME" -config_root "$CONFIG_ROOT"
fi
EOF
    chmod +x "$TARGET_DIR/start.sh"
fi

# Create stop.sh
if [ -f "$SCRIPT_DIR/stop.sh" ]; then
    print_info "Creating stop.sh..."
    cp "$SCRIPT_DIR/stop.sh" "$TARGET_DIR/stop.sh"
    chmod +x "$TARGET_DIR/stop.sh"
else
    print_warn "Source stop.sh not found, creating basic stop.sh..."
    cat > "$TARGET_DIR/stop.sh" << 'EOF'
#!/bin/bash

# AgentSmith-HUB Stop Script

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_info "Stopping AgentSmith-HUB..."

# Find and stop all agentsmith-hub processes
PIDS=$(pgrep -f "agentsmith-hub" 2>/dev/null || true)

if [ -z "$PIDS" ]; then
    print_info "No running AgentSmith-HUB processes found."
    exit 0
fi

print_info "Found running processes: $PIDS"

# Try graceful shutdown first
print_info "Sending TERM signal..."
echo "$PIDS" | xargs kill -TERM 2>/dev/null || true

# Wait a bit for graceful shutdown
sleep 3

# Check if processes are still running
REMAINING=$(pgrep -f "agentsmith-hub" 2>/dev/null || true)
if [ -n "$REMAINING" ]; then
    print_warn "Some processes still running, force killing..."
    echo "$REMAINING" | xargs kill -KILL 2>/dev/null || true
    sleep 1
fi

# Final check
FINAL_CHECK=$(pgrep -f "agentsmith-hub" 2>/dev/null || true)
if [ -z "$FINAL_CHECK" ]; then
    print_info "AgentSmith-HUB stopped successfully."
else
    print_error "Failed to stop some processes."
    exit 1
fi
EOF
    chmod +x "$TARGET_DIR/stop.sh"
fi

# Create README for deployment
print_info "Creating deployment README..."
cat > "$TARGET_DIR/README.md" << EOF
# AgentSmith-HUB Deployment

This directory contains a complete AgentSmith-HUB deployment package.

## Architecture

This package is built for **Linux $ARCHITECTURE** architecture.

## Files

- \`agentsmith-hub\` - Main application binary (Linux $ARCHITECTURE)
- \`web/\` - Frontend web interface files
- \`lib/\` - Required shared libraries ($ARCHITECTURE)
- \`config/\` - Configuration files
- \`start.sh\` - Script to start the services (with architecture detection)
- \`stop.sh\` - Script to stop the services

## Quick Start

1. Extract and enter the directory:
   ```bash
   tar -xzf agentsmith-hub.tar.gz
   cd agentsmith-hub
   ```
2. Start the services:
   ```bash
   ./start.sh
   ```
3. Wait for the services to start (usually takes 10-15 seconds)
4. Open your browser and navigate to: http://localhost:8080
   - You should see the login page
   - Default login token can be configured in `config/config.yaml`
5. To stop the services:
   ```bash
   ./stop.sh
   ```

## Important Notes

- **Login Page**: The web interface should show a login page at startup
- **Backend Status**: Ensure the backend service starts successfully before accessing the web interface
- **Port 8080**: Make sure port 8080 is available and not blocked by firewall
- **Configuration**: Check `config/config.yaml` for authentication and other settings

## Advanced Usage

### Leader Mode (Default)
```bash
./start.sh
```

### Follower Mode
```bash
LEADER_ADDR=<leader_ip:port> ./start.sh --follower <leader_ip:port>
```

### Check Status
```bash
./stop.sh --check
```

## Configuration

Configuration files are located in the `config/` directory. Modify them according to your needs before starting the services.

## Logs

Application logs are written to stdout. To save logs to a file:
```bash
./start.sh > agentsmith-hub.log 2>&1 &
```

## Troubleshooting

If you encounter issues:

1. Check that all files have execute permissions:
   ```bash
   chmod +x agentsmith-hub start.sh stop.sh
   ```

2. Verify library path (if needed):
   ```bash
   export LD_LIBRARY_PATH=$(pwd)/lib:$LD_LIBRARY_PATH
   ```

3. Check configuration files in `config/` directory

For more information, visit: https://github.com/EBWi11/AgentSmith-HUB
EOF

print_info "Deployment scripts created successfully!"
print_info "Target architecture: $ARCHITECTURE"
print_info "Created files:"
print_info "  - $TARGET_DIR/start.sh (with architecture detection)"
print_info "  - $TARGET_DIR/stop.sh"
print_info "  - $TARGET_DIR/README.md (architecture-specific)" 