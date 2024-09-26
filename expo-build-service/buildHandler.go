package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BuildRequest struct {
	RepoURL     string `json:"repo_url"`
	Platform    string `json:"platform"`
	PackagePath string `json:"package_path"`
}

func generateTimestampID() string {
	timestamp := time.Now().Format("20060102-1504") // YearMonthDay-HourMinute
	return timestamp
}

func cloneOrUpdateRepo(ctx context.Context, repoURL, clonePath string) error {
	// Validate input to prevent command injection or path traversal attacks
	if strings.Contains(repoURL, ";") || strings.Contains(repoURL, "&") {
		return fmt.Errorf("invalid repoURL parameter")
	}

	// Clone the repository
	cloneCmd := exec.CommandContext(ctx, "git", "clone", repoURL, clonePath)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error cloning repository: %v, output: %s", err, string(output))
	}

	return nil
}

func buildApp(ctx context.Context, packagePath, platform, outputFile string) error {
	// Validate the platform
	validPlatforms := map[string]bool{"android": true, "ios": true}
	if !validPlatforms[platform] {
		return fmt.Errorf("unsupported platform: %s", platform)
	}

	// Build the app using EAS CLI
	buildCmd := exec.CommandContext(ctx, "eas", "build", "--platform", platform, "--local", "--output", outputFile)
	buildCmd.Dir = packagePath
	buildCmd.Env = os.Environ() // Inherit the environment

	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error building app: %v, output: %s", err, string(output))
	}

	// Check if the built file exists
	builtFilePath := filepath.Join(packagePath, outputFile)
	if _, err := os.Stat(builtFilePath); os.IsNotExist(err) {
		return fmt.Errorf("built app file not found at %s", builtFilePath)
	}

	return nil
}

func runNpmInstall(ctx context.Context, packagePath string) error {
	installCmd := exec.CommandContext(ctx, "npm", "install")
	installCmd.Dir = packagePath
	installCmd.Env = os.Environ() // Inherit the environment

	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error running npm install: %v, output: %s", err, string(output))
	}

	return nil
}

func buildHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Minute)
	defer cancel()

	var req BuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("Invalid request payload:", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.RepoURL == "" || req.Platform == "" || req.PackagePath == "" {
		log.Println("Missing required parameters")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Generate a timestamp-based ID for this build
	buildID := generateTimestampID()

	// Create a temporary directory for this build
	tempDir, err := os.MkdirTemp("", "build-"+buildID)
	if err != nil {
		log.Println("Failed to create temporary directory:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir) // Clean up after build

	clonePath := filepath.Join(tempDir, "repo")

	// Clone the repository
	if err := cloneOrUpdateRepo(ctx, req.RepoURL, clonePath); err != nil {
		log.Println("Failed to clone the repository:", err)
		http.Error(w, "Failed to clone the repository", http.StatusInternalServerError)
		return
	}

	// Run npm install in the package directory
	packagePath := filepath.Join(clonePath, req.PackagePath)
	if err := runNpmInstall(ctx, packagePath); err != nil {
		log.Println("Failed to install npm dependencies:", err)
		http.Error(w, "Failed to install npm dependencies", http.StatusInternalServerError)
		return
	}

	// Define the output file based on the platform and build ID
	var outputFile, contentType, outputFilename string
	switch req.Platform {
	case "android":
		outputFilename = fmt.Sprintf("app-%s.apk", buildID)
		outputFile = outputFilename
		contentType = "application/vnd.android.package-archive"
	case "ios":
		outputFilename = fmt.Sprintf("app-%s.ipa", buildID)
		outputFile = outputFilename
		contentType = "application/octet-stream"
	}

	// Build the app
	if err := buildApp(ctx, packagePath, req.Platform, outputFile); err != nil {
		log.Println("Failed to build the app:", err)
		http.Error(w, "Failed to build the app", http.StatusInternalServerError)
		return
	}

	// Serve the built app
	builtFilePath := filepath.Join(packagePath, outputFile)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outputFilename))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize(builtFilePath)))

	// Stream the file to the client
	file, err := os.Open(builtFilePath)
	if err != nil {
		log.Println("Failed to open built file:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		log.Println("Failed to send file to client:", err)
	}
}

func fileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Println("Failed to get file size:", err)
		return 0
	}
	return info.Size()
}

func main() {
	http.HandleFunc("/build", authenticate(buildHandler))
	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer your-secret-token" {
			log.Println("Unauthorized access attempt")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
