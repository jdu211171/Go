#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status

echo "$(date '+%Y-%m-%d %H:%M:%S') - Starting server update..." | tee -a /home/distro/Go/expo-build-server/update.log

cd /home/distro/Go/expo-build-server

# Pull the latest code
echo "$(date '+%Y-%m-%d %H:%M:%S') - Pulling latest code..." | tee -a update.log
git fetch --all
git reset --hard origin/main  # Replace 'main' with your default branch if different

# Build the new Go executable
echo "$(date '+%Y-%m-%d %H:%M:%S') - Building Go executable..." | tee -a update.log
go build -o buildHandler .

# Restart the server using systemd
echo "$(date '+%Y-%m-%d %H:%M:%S') - Restarting go-server.service..." | tee -a update.log
sudo systemctl restart go-server.service

echo "$(date '+%Y-%m-%d %H:%M:%S') - Server updated and restarted." | tee -a update.log
exit 0
