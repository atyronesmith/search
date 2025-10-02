#!/bin/bash

# Start Unstructured API service

set -e

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
source venv/bin/activate

# Install/upgrade dependencies
echo "Installing dependencies..."
pip install --upgrade pip
pip install -r requirements.txt

# Start the service
echo "Starting Unstructured API on port 8001..."
export UNSTRUCTURED_PORT=8001
export UNSTRUCTURED_HOST=0.0.0.0
python app.py