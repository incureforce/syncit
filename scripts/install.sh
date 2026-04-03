#!/bin/sh

set -e

# Ensure ~/.syncit/bin exists
mkdir -p ~/.syncit/bin

# Detect the current shell and update the appropriate rc file
SHELL_NAME=${SHELL##*/}
RC_FILE=""

if [ "$SHELL_NAME" = "bash" ]; then
    RC_FILE="$HOME/.bashrc"
elif [ "$SHELL_NAME" = "zsh" ]; then
    RC_FILE="$HOME/.zshrc"
else
    echo "Unsupported shell: $SHELL_NAME. Please add 'export PATH=\"\$HOME/.syncit/bin:\$PATH\"' to your shell's rc file manually."
fi

# Add ~/.syncit/bin to PATH if not already present
if [ -n "$RC_FILE" ] && ! grep -q 'syncit/bin' "$RC_FILE"; then
    echo 'export PATH="$HOME/.syncit/bin:$PATH"' >> "$RC_FILE"
fi

# Fetch the latest release tag from GitHub
LATEST_RELEASE=$(curl -fsSL https://api.github.com/repos/incureforce/syncit/releases/latest | awk -F'"' '/"tag_name":/ {print $4; exit}')
if [ -z "$LATEST_RELEASE" ]; then
    echo "Failed to detect latest release tag"
    exit 1
fi

# Create versioned directory
VERSION_DIR="$HOME/.syncit/$LATEST_RELEASE"
mkdir -p "$VERSION_DIR"

# Download and extract the release
echo "Downloading syncit $LATEST_RELEASE..."
curl -sL "https://github.com/incureforce/syncit/releases/download/$LATEST_RELEASE/syncit-linux-amd64.tar.gz" | tar xz -C "$VERSION_DIR"

# Symlink the binary
ln -sf "$VERSION_DIR/syncit-linux-amd64" "$HOME/.syncit/bin/syncit"

echo "syncit $LATEST_RELEASE installed and symlinked to ~/.syncit/bin/syncit"
if [ -n "$RC_FILE" ]; then
    echo "Open a new shell (or run: . $RC_FILE) if PATH was just updated."
fi