#!/bin/bash

SERVER_IP="192.168.0.106"
AUTH_TOKEN="your-secret-token"

# Set to true to trigger server update
UPDATE_SERVER=true

# Temporary file to store the response body
TMP_RESPONSE=$(mktemp)

# Start the build and download the APK
echo "Starting the build and downloading the APK..."

HTTP_STATUS=$(curl -s -w "%{http_code}" \
     -H "Authorization: Bearer $AUTH_TOKEN" \
     -H "Content-Type: application/json" \
     -X POST http://$SERVER_IP:8080/build \
     -d "{
           \"repo_url\": \"https://github.com/jdu211171/parents-monolithic.git\",
           \"platform\": \"android\",
           \"package_path\": \"parent-notification\",
           \"update_server\": $UPDATE_SERVER
         }" \
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
