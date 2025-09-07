#!/bin/bash

# Setup Conda environment for docling with Python 3.11

echo "Setting up Conda environment for Docling..."

# Check if conda is installed
if ! command -v conda &> /dev/null; then
    echo "Conda not found. Installing Miniconda..."
    # Download and install Miniconda
    curl -O https://repo.anaconda.com/miniconda/Miniconda3-latest-MacOSX-x86_64.sh
    bash Miniconda3-latest-MacOSX-x86_64.sh -b -p $HOME/miniconda
    export PATH="$HOME/miniconda/bin:$PATH"
    conda init bash
    source ~/.bashrc
fi

# Create conda environment with Python 3.11
echo "Creating conda environment with Python 3.11..."
conda create -n docling-env python=3.11 -y

# Activate environment
echo "Activating environment..."
source $(conda info --base)/etc/profile.d/conda.sh
conda activate docling-env

# Install docling and dependencies
echo "Installing docling and dependencies..."
pip install --upgrade pip
pip install docling
pip install fastapi uvicorn python-multipart

echo "Setup complete!"
echo ""
echo "To use the environment:"
echo "  conda activate docling-env"
echo "  python app_docling.py"
echo ""
echo "To deactivate:"
echo "  conda deactivate"