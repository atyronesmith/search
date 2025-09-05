#!/bin/bash

# Podman Setup Script for File Search System
# This script helps set up Podman machine on macOS

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Podman Setup Script${NC}"
echo -e "${BLUE}===================${NC}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if we're on macOS
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo -e "${YELLOW}This script is designed for macOS. On Linux, Podman runs natively without a VM.${NC}"
    exit 0
fi

# Check if Podman is installed
if ! command_exists podman; then
    echo -e "${RED}Podman is not installed. Please install it first:${NC}"
    echo -e "${YELLOW}brew install podman${NC}"
    exit 1
fi

PODMAN_VERSION=$(podman --version)
echo -e "${GREEN}Podman is installed: $PODMAN_VERSION${NC}"

# Check if a Podman machine exists
echo -e "\n${BLUE}Checking Podman machine status...${NC}"
MACHINE_EXISTS=$(podman machine list --format json | jq -r '.[0].Name' 2>/dev/null || echo "none")

if [ "$MACHINE_EXISTS" == "none" ] || [ -z "$MACHINE_EXISTS" ]; then
    echo -e "${YELLOW}No Podman machine found. Creating one...${NC}"
    
    # Create Podman machine with reasonable defaults
    echo -e "${BLUE}Creating Podman machine with:${NC}"
    echo "  - CPUs: 2"
    echo "  - Memory: 4GB"
    echo "  - Disk: 20GB"
    
    podman machine init \
        --cpus 2 \
        --memory 4096 \
        --disk-size 20 \
        --now
    
    echo -e "${GREEN}Podman machine created and started${NC}"
else
    echo -e "${GREEN}Podman machine '$MACHINE_EXISTS' already exists${NC}"
    
    # Check if machine is running
    MACHINE_RUNNING=$(podman machine list --format json | jq -r '.[0].Running' 2>/dev/null || echo "false")
    
    if [ "$MACHINE_RUNNING" != "true" ]; then
        echo -e "${YELLOW}Starting Podman machine...${NC}"
        podman machine start
        echo -e "${GREEN}Podman machine started${NC}"
    else
        echo -e "${GREEN}Podman machine is already running${NC}"
    fi
fi

# Test Podman connection
echo -e "\n${BLUE}Testing Podman connection...${NC}"
if podman ps >/dev/null 2>&1; then
    echo -e "${GREEN}Podman is working correctly${NC}"
else
    echo -e "${RED}Failed to connect to Podman. Please check the machine status.${NC}"
    podman machine list
    exit 1
fi

# Check for podman-compose
echo -e "\n${BLUE}Checking podman-compose...${NC}"
if command_exists podman-compose; then
    COMPOSE_VERSION=$(podman-compose --version)
    echo -e "${GREEN}podman-compose is installed: $COMPOSE_VERSION${NC}"
else
    echo -e "${YELLOW}podman-compose is not installed. Installing...${NC}"
    
    if command_exists pip3; then
        pip3 install --user podman-compose
        echo -e "${GREEN}podman-compose installed${NC}"
    else
        echo -e "${YELLOW}Please install podman-compose manually:${NC}"
        echo -e "${YELLOW}pip3 install podman-compose${NC}"
        echo -e "${YELLOW}or${NC}"
        echo -e "${YELLOW}brew install podman-compose${NC}"
    fi
fi

# Set up Podman socket for Docker compatibility (optional)
echo -e "\n${BLUE}Setting up Docker compatibility socket...${NC}"
podman machine ssh -- sudo systemctl enable --now podman.socket 2>/dev/null || true

# Show helpful information
echo -e "\n${GREEN}Podman setup complete!${NC}"
echo -e "\n${BLUE}Useful commands:${NC}"
echo -e "  podman machine list     - Show machine status"
echo -e "  podman machine start    - Start the machine"
echo -e "  podman machine stop     - Stop the machine"
echo -e "  podman machine ssh      - SSH into the machine"
echo -e "  podman ps               - List running containers"
echo -e "\n${BLUE}For the File Search System:${NC}"
echo -e "  make container-up       - Start database container"
echo -e "  make container-down     - Stop database container"
echo -e "  make status             - Check system status"

# Check if we need to pull the pgvector image
echo -e "\n${BLUE}Pre-pulling database image...${NC}"
if podman pull docker.io/ankane/pgvector:latest; then
    echo -e "${GREEN}Database image ready${NC}"
else
    echo -e "${YELLOW}Could not pre-pull image. It will be downloaded on first use.${NC}"
fi

echo -e "\n${GREEN}You're all set! Run 'make setup' to continue with the File Search System setup.${NC}"