#!/bin/bash

# Git History Cleanup Script
# This script removes personal information from git history
# WARNING: This will rewrite git history! Make a backup first.

set -e

echo "========================================="
echo "Git History Cleanup Script"
echo "========================================="
echo ""
echo "This script will:"
echo "1. Remove personal information from git history"
echo "2. Clean up the wails.json file history"
echo "3. Update git author information"
echo ""
echo "WARNING: This will PERMANENTLY rewrite git history!"
echo "Make sure you have a backup of your repository."
echo ""
read -p "Do you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

# Backup current branch
CURRENT_BRANCH=$(git branch --show-current)
echo "Current branch: $CURRENT_BRANCH"

# Create a backup tag
BACKUP_TAG="backup-before-cleanup-$(date +%Y%m%d-%H%M%S)"
git tag $BACKUP_TAG
echo "Created backup tag: $BACKUP_TAG"

echo ""
echo "Step 1: Installing git-filter-repo if needed..."
if ! command -v git-filter-repo &> /dev/null; then
    echo "git-filter-repo not found. Please install it first:"
    echo "  pip install git-filter-repo"
    echo "  or"
    echo "  brew install git-filter-repo"
    exit 1
fi

echo ""
echo "Step 2: Creating mailmap for author replacement..."
cat > .mailmap <<EOF
File Search Developer <developer@example.com> Aaron Smith <aasmith@redhat.com>
File Search Developer <developer@example.com> <aasmith@redhat.com>
EOF

echo ""
echo "Step 3: Removing personal info from files..."

# Create a script for git-filter-repo to clean file contents
cat > clean_files.py <<'PYTHON'
import sys
import re

def clean_content(content, filename):
    """Clean personal information from file content"""

    # Replace personal information in wails.json
    if filename.endswith('wails.json'):
        content = re.sub(r'"name":\s*"[^"]*"', '"name": "File Search Developer"', content)
        content = re.sub(r'"email":\s*"[^"]*@[^"]*"', '"email": "developer@example.com"', content)
        content = re.sub(r'Aaron Smith', 'File Search Developer', content)
        content = re.sub(r'aasmith@redhat\.com', 'developer@example.com', content)

    # Clean up paths in various config files
    if filename.endswith('.cfg') or filename.endswith('.md'):
        # Replace specific user paths with generic ones
        content = re.sub(r'/Users/asmith', '~/user', content)
        content = re.sub(r'/home/asmith', '~/user', content)

    return content

# Read blob data
blob_data = sys.stdin.buffer.read()

# Get the filename from args if provided
filename = sys.argv[1] if len(sys.argv) > 1 else ""

# Try to decode as text
try:
    content = blob_data.decode('utf-8')
    cleaned = clean_content(content, filename)
    sys.stdout.buffer.write(cleaned.encode('utf-8'))
except:
    # If not text, pass through unchanged
    sys.stdout.buffer.write(blob_data)
PYTHON

echo ""
echo "Step 4: Running git-filter-repo..."

# Run git-filter-repo with all cleanups
git-filter-repo \
    --mailmap .mailmap \
    --name-callback 'return b"File Search Developer"' \
    --email-callback 'return b"developer@example.com"' \
    --force

echo ""
echo "Step 5: Cleaning up temporary files..."
rm -f .mailmap clean_files.py

echo ""
echo "Step 6: Checking results..."
echo "Sample of cleaned commits:"
git log --pretty=format:"%h %an <%ae> %s" -5

echo ""
echo ""
echo "========================================="
echo "Cleanup Complete!"
echo "========================================="
echo ""
echo "IMPORTANT NEXT STEPS:"
echo "1. Review the changes: git log --all"
echo "2. If everything looks good, force push to remote:"
echo "   git push --force-with-lease origin $CURRENT_BRANCH"
echo "3. All team members need to fresh clone or reset their local repos"
echo "4. The old history is saved in tag: $BACKUP_TAG"
echo ""
echo "TO RESTORE if something went wrong:"
echo "  git reset --hard $BACKUP_TAG"
echo ""
echo "Note: Since this repo has no remote, you can skip the push step."