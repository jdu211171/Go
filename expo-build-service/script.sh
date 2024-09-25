#!/bin/bash

SERVER_IP="192.168.0.106"
AUTH_TOKEN="your-secret-token"

# Start the build
echo "Starting the build..."
RESPONSE=$(curl -s -H "Authorization: Bearer $AUTH_TOKEN" -X POST http://$SERVER_IP:8080/build)
BUILD_ID=$(echo $RESPONSE | jq -r '.build_id')

# Poll for build status
STATUS="in_progress"
while [ "$STATUS" == "in_progress" ]; do
    sleep 60  # Wait for 30 seconds before checking again
    RESPONSE=$(curl -s -H "Authorization: Bearer $AUTH_TOKEN" http://$SERVER_IP:8080/build/status?build_id=$BUILD_ID)
    STATUS=$(echo $RESPONSE | jq -r '.status')
    ERROR=$(echo $RESPONSE | jq -r '.error')
    echo "Build status: $STATUS"
    if [ "$STATUS" == "error" ]; then
        echo "Build failed with error: $ERROR"
        exit 1
    fi
done

# Download the APK
if [ "$STATUS" == "completed" ]; then
    echo "Downloading the APK..."
    curl -H "Authorization: Bearer $AUTH_TOKEN" http://$SERVER_IP:8080/build/download?build_id=$BUILD_ID -o app.apk
    echo "APK downloaded as app.apk"
else
    echo "Unexpected build status: $STATUS"
    exit 1
fi
