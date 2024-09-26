#!/bin/bash

SERVER_IP="192.168.0.106"
AUTH_TOKEN="your-secret-token"

# Temporary file to store the response body
TMP_RESPONSE=$(mktemp)

# Start the build and download the APK
echo "Starting the build and downloading the APK..."

HTTP_STATUS=$(curl -s -w "%{http_code}" \
     -H "Authorization: Bearer $AUTH_TOKEN" \
     -H "Content-Type: application/json" \
     -X POST http://$SERVER_IP:8080/build \
     -d '{
           "repo_url": "https://github.com/yourusername/your-repo.git",
           "platform": "android"
         }' \
     -o app.apk)

if [ "$HTTP_STATUS" -eq 200 ]; then
    echo "APK downloaded as app.apk"
    rm -f $TMP_RESPONSE
else
    echo "Failed to build the app. HTTP status code: $HTTP_STATUS"
    echo "Server response:"
    cat app.apk  # Output the server's error message
    rm -f app.apk
    exit 1
fi
