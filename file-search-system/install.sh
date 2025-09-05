#!/bin/bash

# File Search System - Installation Script
# This script helps set up the development environment

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}File Search System - Installation Script${NC}"
echo -e "${BLUE}=======================================${NC}"

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: Please run this script from the file-search-system root directory${NC}"
    exit 1
fi

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install with package manager
install_package() {
    local package=$1
    local package_manager=""
    
    if command_exists brew; then
        package_manager="brew"
    elif command_exists apt-get; then
        package_manager="apt-get"
    elif command_exists yum; then
        package_manager="yum"
    elif command_exists dnf; then
        package_manager="dnf"
    elif command_exists pacman; then
        package_manager="pacman"
    fi
    
    case $package_manager in
        brew)
            echo -e "${YELLOW}Installing $package with Homebrew...${NC}"
            brew install "$package"
            ;;
        apt-get)
            echo -e "${YELLOW}Installing $package with apt-get...${NC}"
            sudo apt-get update && sudo apt-get install -y "$package"
            ;;
        yum)
            echo -e "${YELLOW}Installing $package with yum...${NC}"
            sudo yum install -y "$package"
            ;;
        dnf)
            echo -e "${YELLOW}Installing $package with dnf...${NC}"
            sudo dnf install -y "$package"
            ;;
        pacman)
            echo -e "${YELLOW}Installing $package with pacman...${NC}"
            sudo pacman -S --noconfirm "$package"
            ;;
        *)
            echo -e "${RED}No supported package manager found. Please install $package manually.${NC}"
            return 1
            ;;
    esac
}

# Check and install Go
echo -e "\n${BLUE}Checking Go installation...${NC}"
if command_exists go; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "${GREEN}Go is installed: $GO_VERSION${NC}"
else
    echo -e "${YELLOW}Go is not installed.${NC}"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo -e "${YELLOW}Installing Go with Homebrew...${NC}"
        brew install go
    else
        echo -e "${RED}Please install Go manually from https://golang.org/dl/${NC}"
        exit 1
    fi
fi

# Check and install Node.js
echo -e "\n${BLUE}Checking Node.js installation...${NC}"
if command_exists node; then
    NODE_VERSION=$(node --version)
    echo -e "${GREEN}Node.js is installed: $NODE_VERSION${NC}"
else
    echo -e "${YELLOW}Node.js is not installed.${NC}"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo -e "${YELLOW}Installing Node.js with Homebrew...${NC}"
        brew install node
    else
        echo -e "${RED}Please install Node.js manually from https://nodejs.org/${NC}"
        exit 1
    fi
fi

# Check for container runtime (Podman preferred, Docker as fallback)
echo -e "\n${BLUE}Checking container runtime...${NC}"
CONTAINER_RUNTIME=""
COMPOSE_CMD=""

# Check for Podman first (preferred)
if command_exists podman; then
    PODMAN_VERSION=$(podman --version)
    echo -e "${GREEN}Podman is installed: $PODMAN_VERSION${NC}"
    CONTAINER_RUNTIME="podman"
    
    # Check if Podman is running/accessible
    if podman ps >/dev/null 2>&1; then
        echo -e "${GREEN}Podman is running${NC}"
    else
        echo -e "${YELLOW}Podman is installed but not accessible. You may need to run 'podman machine start'${NC}"
    fi
    
    # Check for podman-compose
    if command_exists podman-compose; then
        echo -e "${GREEN}podman-compose is installed${NC}"
        COMPOSE_CMD="podman-compose"
    else
        echo -e "${YELLOW}podman-compose is not installed. Installing...${NC}"
        if command_exists pip3; then
            pip3 install --user podman-compose
            COMPOSE_CMD="podman-compose"
        else
            echo -e "${YELLOW}pip3 not found. Please install podman-compose manually: pip3 install podman-compose${NC}"
        fi
    fi
# Fall back to Docker if Podman is not available
elif command_exists docker; then
    DOCKER_VERSION=$(docker --version)
    echo -e "${YELLOW}Docker is installed (Podman recommended): $DOCKER_VERSION${NC}"
    CONTAINER_RUNTIME="docker"
    
    # Check if Docker is running
    if docker ps >/dev/null 2>&1; then
        echo -e "${GREEN}Docker is running${NC}"
    else
        echo -e "${YELLOW}Docker is installed but not running. Please start Docker.${NC}"
    fi
    
    if command_exists docker-compose; then
        COMPOSE_CMD="docker-compose"
    else
        echo -e "${YELLOW}docker-compose not found${NC}"
    fi
else
    echo -e "${YELLOW}No container runtime found. Installing Podman...${NC}"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo -e "${YELLOW}Installing Podman with Homebrew...${NC}"
        brew install podman
        brew install podman-compose
        echo -e "${YELLOW}Please run 'podman machine init' and 'podman machine start' to set up Podman${NC}"
        CONTAINER_RUNTIME="podman"
        COMPOSE_CMD="podman-compose"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command_exists apt-get; then
            echo -e "${YELLOW}Installing Podman with apt...${NC}"
            sudo apt-get update && sudo apt-get install -y podman
        elif command_exists dnf; then
            echo -e "${YELLOW}Installing Podman with dnf...${NC}"
            sudo dnf install -y podman
        elif command_exists yum; then
            echo -e "${YELLOW}Installing Podman with yum...${NC}"
            sudo yum install -y podman
        else
            echo -e "${RED}Please install Podman manually from https://podman.io/getting-started/installation${NC}"
            exit 1
        fi
        
        # Install podman-compose
        if command_exists pip3; then
            pip3 install --user podman-compose
        fi
        CONTAINER_RUNTIME="podman"
        COMPOSE_CMD="podman-compose"
    else
        echo -e "${RED}Please install Podman manually from https://podman.io/getting-started/installation${NC}"
        exit 1
    fi
fi

# Export for Makefile to use
export CONTAINER_RUNTIME
export COMPOSE_CMD
echo -e "${GREEN}Using container runtime: $CONTAINER_RUNTIME${NC}"
echo -e "${GREEN}Using compose command: $COMPOSE_CMD${NC}"

# Check and install PostgreSQL client
echo -e "\n${BLUE}Checking PostgreSQL client...${NC}"
if command_exists psql; then
    PSQL_VERSION=$(psql --version)
    echo -e "${GREEN}PostgreSQL client is installed: $PSQL_VERSION${NC}"
else
    echo -e "${YELLOW}PostgreSQL client is not installed.${NC}"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo -e "${YELLOW}Installing PostgreSQL with Homebrew...${NC}"
        brew install postgresql
    else
        install_package "postgresql-client"
    fi
fi

# Install development tools
echo -e "\n${BLUE}Installing Go development tools...${NC}"
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/tools/cmd/goimports@latest
echo -e "${GREEN}Go tools installed${NC}"

# Copy environment file
echo -e "\n${BLUE}Setting up environment configuration...${NC}"
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo -e "${GREEN}Created .env file from template${NC}"
    echo -e "${YELLOW}Please review and modify .env file as needed${NC}"
else
    echo -e "${YELLOW}.env file already exists${NC}"
fi

# Run make setup
echo -e "\n${BLUE}Running initial setup...${NC}"
if make setup; then
    echo -e "\n${GREEN}Installation completed successfully!${NC}"
    echo -e "\n${BLUE}Next steps:${NC}"
    echo -e "1. Review and modify the .env file if needed"
    echo -e "2. Run 'make dev' to start the development environment"
    echo -e "3. Run 'make help' to see all available commands"
    echo -e "\n${BLUE}Quick commands:${NC}"
    echo -e "  make dev      - Start development environment"
    echo -e "  make status   - Check system status"
    echo -e "  make help     - Show all available commands"
else
    echo -e "\n${RED}Setup failed. Please check the output above for errors.${NC}"
    echo -e "You can try running 'make setup' manually to see detailed error messages."
    exit 1
fi