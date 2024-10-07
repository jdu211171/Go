#!/bin/bash

SERVER_IP="10.20.13.221"  # Replace with your server's IP address
AUTH_TOKEN="your-secret-token"  # Replace with your actual token

# Function to trigger server update
trigger_update() {
    echo "Triggering server update..."
    HTTP_STATUS=$(curl -s -w "%{http_code}" \
         -H "Authorization: Bearer $AUTH_TOKEN" \
         -X GET http://$SERVER_IP:8080/update \
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
         -H "Content-Type: application/json" \
         -H "Authorization: Bearer $AUTH_TOKEN" \
         -X POST http://$SERVER_IP:8080/build \
         -d '{
               "repo_url": "https://github.com/jdu211171/parents-monolithic.git",
               "platform": "android",
               "package_path": "parent-notification"
             }' \
         -D - \
         -o $TMP_RESPONSE)

    # Extract the HTTP status code
    HTTP_CODE=$(echo "$HTTP_STATUS" | tail -n1)

    if [ "$HTTP_CODE" -eq 200 ]; then
        # Extract the filename from the Content-Disposition header
        FILENAME=$(grep -i -E 'Content-Disposition:.*filename="[^"]+"' $TMP_RESPONSE | sed 's/Content-Disposition: .*filename="//;s/"//')

        if [ -z "$FILENAME" ]; then
            FILENAME="app.apk"
        fi

        # Move the temporary response file to the final filename
        mv $TMP_RESPONSE $FILENAME
        echo "APK downloaded as $FILENAME"
    else
        echo "Failed to build the app. HTTP status code: $HTTP_CODE"
        echo "Server response:"
        cat $TMP_RESPONSE  # Output the server's error message
        rm -f $TMP_RESPONSE
        exit 1
    fi
}

# Execute the functions
# trigger_update

# Wait for the server to restart
# echo "Waiting for the server to restart..."
# sleep 60  # Adjust the sleep duration based on your update process

# Build and download the APK
build_and_download

# curl -H "Authorization: Bearer your-secret-token" http://192.168.0.106:8080/update
