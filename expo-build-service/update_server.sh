#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status

echo "Starting server update..."

cd /home/distro/Go/expo-build-server  # Ensure you are in the correct directory

# Pull the latest code
git fetch --all
git reset --hard origin/main  # Replace 'main' with your default branch if different

# Build the new Go executable
go build -o buildHandler .

# Restart the server using systemd
sudo systemctl restart go-server.service

echo "Server updated and restarted."
exit 0
