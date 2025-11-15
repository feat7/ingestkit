#!/usr/bin/env bash
#
# IngestKit System Tuning Script
# Tunes the host system for high-performance load testing
#
# Usage: ./scripts/tune-system.sh
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}IngestKit System Tuning for Load Testing${NC}"
echo ""

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS_TYPE=Linux;;
    Darwin*)    OS_TYPE=macOS;;
    *)          OS_TYPE="UNKNOWN:${OS}"
esac

echo -e "${YELLOW}Detected OS: ${OS_TYPE}${NC}"
echo ""

# Function to check if running as root (for Linux)
check_root() {
    if [ "$OS_TYPE" = "Linux" ] && [ "$EUID" -ne 0 ]; then
        echo -e "${RED}Error: This script must be run as root on Linux${NC}"
        echo "Please run: sudo $0"
        exit 1
    fi
}

# macOS Tuning
tune_macos() {
    echo -e "${BLUE}Tuning macOS for load testing...${NC}"
    echo ""

    # Check current file descriptor limits
    echo -e "${YELLOW}Current file descriptor limits:${NC}"
    sysctl kern.maxfiles kern.maxfilesperproc 2>/dev/null || true
    echo ""

    # Check if limits need adjustment
    CURRENT_MAXFILES=$(sysctl -n kern.maxfiles 2>/dev/null || echo 0)
    CURRENT_MAXFILESPERPROC=$(sysctl -n kern.maxfilesperproc 2>/dev/null || echo 0)

    if [ "$CURRENT_MAXFILES" -lt 200000 ] || [ "$CURRENT_MAXFILESPERPROC" -lt 100000 ]; then
        echo -e "${YELLOW}File descriptor limits are below recommended values${NC}"
        echo -e "${YELLOW}Recommended: kern.maxfiles=200000 kern.maxfilesperproc=100000${NC}"
        echo ""
        echo -e "${YELLOW}To increase limits (requires root):${NC}"
        echo "  sudo sysctl -w kern.maxfiles=200000"
        echo "  sudo sysctl -w kern.maxfilesperproc=100000"
        echo ""
        echo -e "${YELLOW}To make permanent, add to /etc/sysctl.conf:${NC}"
        echo "  kern.maxfiles=200000"
        echo "  kern.maxfilesperproc=100000"
        echo ""
    else
        echo -e "${GREEN}✓ File descriptor limits are sufficient${NC}"
    fi

    # Check ulimit
    echo -e "${YELLOW}Checking shell ulimit...${NC}"
    ULIMIT=$(ulimit -n)
    echo "Current ulimit -n: $ULIMIT"

    if [ "$ULIMIT" != "unlimited" ] && [ "$ULIMIT" -lt 65536 ]; then
        echo -e "${YELLOW}Shell ulimit is low, setting to 65536${NC}"
        ulimit -n 65536 || echo -e "${RED}Failed to set ulimit (you may need to adjust in your shell profile)${NC}"
    else
        echo -e "${GREEN}✓ Shell ulimit is sufficient${NC}"
    fi
    echo ""

    # Docker Desktop for Mac
    echo -e "${YELLOW}Docker Desktop for Mac Settings:${NC}"
    echo "Ensure Docker Desktop has sufficient resources:"
    echo "  • CPUs: 6-8 cores minimum"
    echo "  • Memory: 8GB minimum (12GB+ recommended)"
    echo "  • Swap: 2GB minimum"
    echo "  • Disk: 60GB minimum"
    echo ""
    echo "To check: Docker Desktop → Preferences → Resources"
    echo ""
}

# Linux Tuning
tune_linux() {
    echo -e "${BLUE}Tuning Linux for load testing...${NC}"
    echo ""

    # File descriptor limits
    echo -e "${YELLOW}Setting file descriptor limits...${NC}"

    # Set system-wide limits
    sysctl -w fs.file-max=2097152 || true
    sysctl -w fs.nr_open=2097152 || true

    # Set per-process limits
    cat > /etc/security/limits.d/99-ingestkit.conf <<EOF
*    soft nofile 65536
*    hard nofile 65536
*    soft nproc  32768
*    hard nproc  32768
root soft nofile 65536
root hard nofile 65536
EOF

    echo -e "${GREEN}✓ File descriptor limits set${NC}"
    echo ""

    # Network tuning
    echo -e "${YELLOW}Tuning network stack...${NC}"

    sysctl -w net.core.somaxconn=65535 || true
    sysctl -w net.ipv4.tcp_max_syn_backlog=65535 || true
    sysctl -w net.core.netdev_max_backlog=65535 || true
    sysctl -w net.ipv4.ip_local_port_range="1024 65535" || true
    sysctl -w net.ipv4.tcp_tw_reuse=1 || true
    sysctl -w net.ipv4.tcp_fin_timeout=30 || true
    sysctl -w net.ipv4.tcp_keepalive_time=600 || true
    sysctl -w net.ipv4.tcp_keepalive_intvl=30 || true
    sysctl -w net.ipv4.tcp_keepalive_probes=3 || true

    echo -e "${GREEN}✓ Network stack tuned${NC}"
    echo ""

    # Virtual memory tuning
    echo -e "${YELLOW}Tuning virtual memory...${NC}"

    sysctl -w vm.swappiness=10 || true
    sysctl -w vm.dirty_ratio=15 || true
    sysctl -w vm.dirty_background_ratio=5 || true
    sysctl -w vm.overcommit_memory=1 || true

    echo -e "${GREEN}✓ Virtual memory tuned${NC}"
    echo ""

    # Make permanent
    echo -e "${YELLOW}Making settings permanent...${NC}"
    cat > /etc/sysctl.d/99-ingestkit.conf <<EOF
# IngestKit Load Testing Tuning
fs.file-max=2097152
fs.nr_open=2097152
net.core.somaxconn=65535
net.ipv4.tcp_max_syn_backlog=65535
net.core.netdev_max_backlog=65535
net.ipv4.ip_local_port_range=1024 65535
net.ipv4.tcp_tw_reuse=1
net.ipv4.tcp_fin_timeout=30
net.ipv4.tcp_keepalive_time=600
net.ipv4.tcp_keepalive_intvl=30
net.ipv4.tcp_keepalive_probes=3
vm.swappiness=10
vm.dirty_ratio=15
vm.dirty_background_ratio=5
vm.overcommit_memory=1
EOF

    echo -e "${GREEN}✓ Settings saved to /etc/sysctl.d/99-ingestkit.conf${NC}"
    echo ""
}

# Docker tuning (cross-platform)
tune_docker() {
    echo -e "${BLUE}Checking Docker configuration...${NC}"
    echo ""

    # Check if Docker is running
    if ! docker info > /dev/null 2>&1; then
        echo -e "${RED}Docker is not running. Please start Docker first.${NC}"
        return 1
    fi

    # Show Docker resources
    echo -e "${YELLOW}Docker system info:${NC}"
    docker info --format '{{.NCPU}} CPUs, {{.MemTotal}} memory' 2>/dev/null || docker info | grep -E "CPUs|Total Memory"
    echo ""

    # Check Docker Compose version
    echo -e "${YELLOW}Docker Compose version:${NC}"
    docker compose version 2>/dev/null || docker-compose --version
    echo ""

    echo -e "${GREEN}✓ Docker configuration checked${NC}"
    echo ""
}

# Display recommendations
show_recommendations() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}System Tuning Complete!${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${YELLOW}Next steps for load testing:${NC}"
    echo ""
    echo "  1. Start optimized stack:"
    echo -e "     ${BLUE}make loadtest-start${NC}"
    echo ""
    echo "  2. Wait for services to be healthy (30-60 seconds)"
    echo ""
    echo "  3. Run load test:"
    echo -e "     ${BLUE}make loadtest-high${NC}      # 10,000 RPS"
    echo -e "     ${BLUE}make loadtest-baseline${NC}  # 500 RPS baseline"
    echo ""
    echo "  4. Monitor metrics:"
    echo -e "     ${BLUE}make metrics-watch${NC}       # Consumer metrics"
    echo -e "     ${BLUE}make db-stats${NC}            # Database stats"
    echo ""
    echo "  5. Stop when done:"
    echo -e "     ${BLUE}make loadtest-stop${NC}"
    echo ""
    echo -e "${YELLOW}Monitoring URLs:${NC}"
    echo "  • API:              http://localhost:8080/health"
    echo "  • Consumer:         http://localhost:8081/metrics"
    echo "  • Redpanda Console: http://localhost:8090"
    echo ""
}

# Main execution
main() {
    case "${OS_TYPE}" in
        Linux)
            check_root
            tune_linux
            ;;
        macOS)
            tune_macos
            ;;
        *)
            echo -e "${RED}Unsupported OS: ${OS_TYPE}${NC}"
            exit 1
            ;;
    esac

    tune_docker
    show_recommendations
}

main
