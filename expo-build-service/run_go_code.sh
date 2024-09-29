#!/bin/bash

# Variables
GO_SERVER_DIR="/home/distro/Go/expo-build-service"
GO_EXECUTABLE="buildHandler"

# Ensure the script is executed from the correct directory
cd "$GO_SERVER_DIR" || { echo "Failed to change directory to $GO_SERVER_DIR"; exit 1; }

# Run the Go executable
echo "Running Go executable..."
./"$GO_EXECUTABLE"