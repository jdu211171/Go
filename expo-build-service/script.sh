#!/bin/bash

SERVER_IP="192.168.0.106"
AUTH_TOKEN="your-secret-token"

# Function to trigger server update
trigger_update() {
    echo "Triggering server update..."
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
         -H "Authorization: Bearer $AUTH_TOKEN" \
         -H "Content-Type: application/json" \
         -X POST http://$SERVER_IP:8080/build \
         -d '{
               "repo_url": "https://github.com/jdu211171/parents-monolithic.git",
               "platform": "android",
               "package_path": "parent-notification",
               "update_server": true
             }' \
         -o /dev/null)

    if [ "$HTTP_STATUS" -eq 200 ]; then
        echo "Server update initiated successfully."
    else
        echo "Failed to initiate server update. HTTP status code: $HTTP_STATUS"
        exit 1
    fi
}

# Function to build and download APK
build_and_download() {
    TMP_RESPONSE=$(mktemp)
    echo "Starting the build and downloading the APK..."

    HTTP_STATUS=$(curl -s -w "%{http_code}" \
         -H "Authorization: Bearer $AUTH_TOKEN" \
         -H "Content-Type: application/json" \
         -X POST http://$SERVER_IP:8080/build \
         -d '{
               "repo_url": "https://github.com/jdu211171/parents-monolithic.git",
               "platform": "android",
               "package_path": "parent-notification",
               "update_server": false
             }' \
         -o $TMP_RESPONSE)

    if [ "$HTTP_STATUS" -eq 200 ]; then
        # Extract the filename from the Content-Disposition header
        FILENAME=$(grep -o -E 'filename="[^"]+"' $TMP_RESPONSE | sed 's/filename="//;s/"//')
        if [ -z "$FILENAME" ]; then
            FILENAME="app.apk"
        fi

        # Move the temporary response file to the final filename
        mv $TMP_RESPONSE $FILENAME
        echo "APK downloaded as $FILENAME"
    else
        echo "Failed to build the app. HTTP status code: $HTTP_STATUS"
        echo "Server response:"
        cat $TMP_RESPONSE  # Output the server's error message
        rm -f $TMP_RESPONSE
        exit 1
    fi
}

# Trigger server update
trigger_update

# Wait for the server to restart
echo "Waiting for the server to restart..."
sleep 60  # Adjust the sleep duration as needed

# Build and download the APK
build_and_download
