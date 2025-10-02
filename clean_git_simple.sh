#!/bin/bash

# Simple Git History Cleanup Script
# Uses built-in git commands to clean author information

set -e

echo "========================================="
echo "Simple Git Author Cleanup Script"
echo "========================================="
echo ""
echo "This script will change all git commit authors to anonymous."
echo "WARNING: This will rewrite git history!"
echo ""
read -p "Do you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

# Create backup
BACKUP_TAG="backup-$(date +%Y%m%d-%H%M%S)"
git tag $BACKUP_TAG
echo "Created backup tag: $BACKUP_TAG"

echo ""
echo "Rewriting git history to anonymize authors..."

# Use git filter-branch to rewrite history
git filter-branch --env-filter '
    export GIT_AUTHOR_NAME="File Search Developer"
    export GIT_AUTHOR_EMAIL="developer@example.com"
    export GIT_COMMITTER_NAME="File Search Developer"
    export GIT_COMMITTER_EMAIL="developer@example.com"
' --tag-name-filter cat -- --all

echo ""
echo "Cleaning up..."
# Remove original refs left by filter-branch
git for-each-ref --format="%(refname)" refs/original/ | xargs -I {} git update-ref -d {}

echo ""
echo "Results:"
git log --pretty=format:"%h %an <%ae> %s" -5

echo ""
echo "========================================="
echo "Cleanup Complete!"
echo "========================================="
echo ""
echo "The old history is saved in tag: $BACKUP_TAG"
echo ""
echo "TO RESTORE if needed:"
echo "  git reset --hard $BACKUP_TAG"
echo ""
echo "TO REMOVE backup refs completely:"
echo "  git tag -d $BACKUP_TAG"
echo "  git gc --prune=now --aggressive"