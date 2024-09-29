#!/bin/bash

# Variables
SERVICE_FILE="expo-build-service/go-server.service"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_NAME="go-server.service"
GO_SERVER_DIR="/home/distro/Go/expo-build-service"
GO_EXECUTABLE="buildHandler"

# Ensure the script is executed from the correct directory
cd "$GO_SERVER_DIR" || { echo "Failed to change directory to $GO_SERVER_DIR"; exit 1; }

# Copy the systemd service file to the systemd directory
echo "Copying systemd service file..."
sudo cp "$SERVICE_FILE" "$SYSTEMD_DIR"

# Reload systemd to recognize the new service
echo "Reloading systemd daemon..."
sudo systemctl daemon-reload

# Enable the service to start on boot
echo "Enabling $SERVICE_NAME..."
sudo systemctl enable "$SERVICE_NAME"

# Start the service
echo "Starting $SERVICE_NAME..."
sudo systemctl start "$SERVICE_NAME"

# Check the status of the service
echo "Checking the status of $SERVICE_NAME..."
sudo systemctl status "$SERVICE_NAME"

# Build the Go executable
echo "Building Go executable..."
go build -o "$GO_EXECUTABLE" .

echo "Setup completed successfully."