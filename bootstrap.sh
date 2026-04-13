#!/bin/bash
# Exit immediately if a command exits with a non-zero status.
set -e

echo "========================================"
echo "  Initializing Docksmith Environment    "
echo "========================================"

# 1. Setup the secret directories
echo "-> Creating local storage (~/.docksmith)..."
mkdir -p ~/.docksmith/layers
mkdir -p ~/.docksmith/images

# 2. Download the official Alpine Linux Mini Root Filesystem
echo "-> Fetching Alpine 3.18 minirootfs..."
wget -qO alpine-dl.tar.gz https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-minirootfs-3.18.4-x86_64.tar.gz

# 3. Unpack the compressed gz file into a temporary folder
echo "-> Unpacking rootfs..."
mkdir -p temp_rootfs
tar -xf alpine-dl.tar.gz -C temp_rootfs
rm alpine-dl.tar.gz # Remove the original download

# 4. Re-compress it using standard tar (No GZIP allowed in Docksmith layers!)
echo "-> Standardizing Docksmith base layer..."
tar -cf alpine-base.tar -C temp_rootfs .
rm -rf temp_rootfs # Clean up the temp folder

# 5. Calculate the SHA-256 hash (cross-platform awk to just grab the hash string)
echo "-> Calculating SHA-256 Digest..."
HASH=$(sha256sum alpine-base.tar | awk '{print $1}')
echo "   Base Hash: sha256:${HASH}"

# 6. Move the finalized layer into the cache
echo "-> Injecting into local cache..."
mv alpine-base.tar ~/.docksmith/layers/${HASH}.tar

echo "========================================"
echo "  Environment Ready! Starting Build...  "
echo "========================================"

# 7. Pass the hash to Docksmith via Environment Variable and trigger the build!
export DOCKSMITH_BASE_HASH="sha256:${HASH}"

# Run the executable that lives in the same directory as this script
./docksmith build -t myapp:latest .

echo ""
echo "-> Build Complete. You can now run:"
echo "   ./docksmith run myapp:latest"
