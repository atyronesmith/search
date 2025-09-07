#!/bin/bash

# Start script for docling service
# Can run either with conda (local) or podman (containerized)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Function to check if conda environment exists
check_conda_env() {
    if [[ -z "$CONDA_PREFIX" ]]; then
        if [[ -f "$HOME/miniconda/bin/conda" ]]; then
            export PATH="$HOME/miniconda/bin:$PATH"
        fi
    fi
    
    if command -v conda >/dev/null 2>&1; then
        conda env list | grep -q docling-env
        return $?
    fi
    return 1
}

# Function to start with conda
start_with_conda() {
    echo "Starting docling service with conda environment..."
    
    # Source conda
    if [[ -f "$HOME/miniconda/etc/profile.d/conda.sh" ]]; then
        source "$HOME/miniconda/etc/profile.d/conda.sh"
    elif [[ -f "$HOME/.bash_profile" ]]; then
        source "$HOME/.bash_profile"
    fi
    
    # Activate environment
    conda activate docling-env
    
    # Verify docling works
    python -c "from docling.document_converter import DocumentConverter; print('Docling ready!')" || {
        echo "Error: Docling not working in conda environment"
        exit 1
    }
    
    # Start the service
    echo "Starting docling service on port 8082..."
    python app_docling.py
}

# Function to start with podman
start_with_podman() {
    echo "Starting docling service with podman..."
    
    # Check if Containerfile exists
    if [[ ! -f "Containerfile" ]]; then
        echo "Error: Containerfile not found"
        exit 1
    fi
    
    # Build image if it doesn't exist
    if ! podman image exists localhost/docling-service:latest; then
        echo "Building docling service image..."
        podman build -t docling-service -f Containerfile .
    fi
    
    # Stop existing container if running
    if podman ps -a | grep -q docling-service; then
        echo "Stopping existing docling service container..."
        podman stop docling-service 2>/dev/null || true
        podman rm docling-service 2>/dev/null || true
    fi
    
    # Start container
    echo "Starting docling service container on port 8082..."
    podman run -d \
        --name docling-service \
        --publish 8082:8082 \
        --volume /tmp:/tmp:rw \
        --volume "$(pwd)/downloads":/downloads:ro \
        --env PYTHONUNBUFFERED=1 \
        --env HF_HOME=/tmp/huggingface \
        --env TRANSFORMERS_CACHE=/tmp/transformers \
        --env TORCH_HOME=/tmp/torch \
        --restart unless-stopped \
        localhost/docling-service:latest
    
    echo "Container started. Checking logs..."
    sleep 5
    podman logs docling-service
}

# Function to show status
show_status() {
    echo "=== Docling Service Status ==="
    
    # Check conda environment
    if check_conda_env; then
        echo "✓ Conda environment 'docling-env' exists"
    else
        echo "✗ Conda environment 'docling-env' not found"
    fi
    
    # Check if service is running (conda)
    if pgrep -f "python.*app_docling.py" >/dev/null; then
        echo "✓ Docling service running (conda)"
        echo "  PID: $(pgrep -f 'python.*app_docling.py')"
    else
        echo "✗ Docling service not running (conda)"
    fi
    
    # Check podman container
    if command -v podman >/dev/null 2>&1; then
        if podman ps | grep -q docling-service; then
            echo "✓ Docling service container running (podman)"
            podman ps --filter name=docling-service --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        else
            echo "✗ Docling service container not running (podman)"
        fi
    else
        echo "✗ Podman not available"
    fi
    
    # Test connectivity
    echo ""
    echo "Testing connectivity..."
    if curl -s http://localhost:8082/health >/dev/null 2>&1; then
        echo "✓ Service responding on http://localhost:8082"
    else
        echo "✗ Service not responding on http://localhost:8082"
    fi
}

# Main script
case "${1:-start}" in
    start)
        # Prefer conda for development, podman for production
        if check_conda_env; then
            start_with_conda
        else
            echo "Conda environment not found, falling back to podman..."
            start_with_podman
        fi
        ;;
    conda)
        start_with_conda
        ;;
    podman)
        start_with_podman
        ;;
    status)
        show_status
        ;;
    stop)
        echo "Stopping docling services..."
        # Stop conda process
        if pgrep -f "python.*app_docling.py" >/dev/null; then
            pkill -f "python.*app_docling.py"
            echo "Stopped conda process"
        fi
        # Stop podman container
        if command -v podman >/dev/null 2>&1 && podman ps | grep -q docling-service; then
            podman stop docling-service
            echo "Stopped podman container"
        fi
        ;;
    help)
        echo "Usage: $0 {start|conda|podman|status|stop|help}"
        echo ""
        echo "Commands:"
        echo "  start   - Start service (auto-detect conda/podman)"
        echo "  conda   - Start with conda environment"
        echo "  podman  - Start with podman container"
        echo "  status  - Show service status"
        echo "  stop    - Stop all docling services"
        echo "  help    - Show this help message"
        ;;
    *)
        echo "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac