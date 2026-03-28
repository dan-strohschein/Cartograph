#!/bin/sh
# Release script for Cartograph
# Automatically increments the version, tags, and pushes to trigger
# the GitHub Actions release workflow that builds binaries for:
#   linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64
#
# Usage:
#   ./release.sh           # auto-increment patch (v0.1.0 → v0.1.1)
#   ./release.sh minor     # auto-increment minor (v0.1.1 → v0.2.0)
#   ./release.sh major     # auto-increment major (v0.2.0 → v1.0.0)
#   ./release.sh v1.2.3    # use explicit version
set -e

BUMP="${1:-patch}"

# Get the latest version tag
LATEST=$(git tag -l 'v*' --sort=-v:refname | head -1)
if [ -z "$LATEST" ]; then
  LATEST="v0.0.0"
fi

echo "Current version: ${LATEST}"

# Parse major.minor.patch
MAJOR=$(echo "$LATEST" | sed 's/^v//' | cut -d. -f1)
MINOR=$(echo "$LATEST" | sed 's/^v//' | cut -d. -f2)
PATCH=$(echo "$LATEST" | sed 's/^v//' | cut -d. -f3)

# Compute next version
case "$BUMP" in
  patch)
    PATCH=$((PATCH + 1))
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  v*)
    # Explicit version provided
    NEXT="$BUMP"
    ;;
  *)
    echo "Usage: $0 [patch|minor|major|vX.Y.Z]"
    exit 1
    ;;
esac

if [ -z "$NEXT" ]; then
  NEXT="v${MAJOR}.${MINOR}.${PATCH}"
fi

# Safety checks
if git tag -l "$NEXT" | grep -q .; then
  echo "Error: tag ${NEXT} already exists"
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "Error: working tree is dirty. Commit or stash changes first."
  exit 1
fi

# Confirm
echo "Next version: ${NEXT}"
printf "Tag and push %s to trigger release build? [y/N] " "$NEXT"
read -r CONFIRM
case "$CONFIRM" in
  y|Y|yes|YES) ;;
  *) echo "Aborted."; exit 0 ;;
esac

# Tag and push
git tag -a "$NEXT" -m "Release ${NEXT}"
git push origin "$NEXT"

echo ""
echo "Tagged ${NEXT} and pushed. Release workflow triggered."
echo "Watch progress: https://github.com/dan-strohschein/Cartograph/actions"
echo "Release will appear at: https://github.com/dan-strohschein/Cartograph/releases/tag/${NEXT}"
