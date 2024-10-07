package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Mutex to prevent concurrent updates
var updateMutex sync.Mutex
var updateInProgress bool

// BuildRequest defines the expected JSON payload for build requests
type BuildRequest struct {
	RepoURL      string `json:"repo_url"`
	Platform     string `json:"platform"`
	PackagePath  string `json:"package_path"`
	UpdateServer bool   `json:"update_server"`
}

// Generate a timestamp-based ID for builds
func generateTimestampID() string {
	timestamp := time.Now().Format("20060102-1504") // YearMonthDay-HourMinute
	return timestamp
}

// Clone or update the repository
func cloneOrUpdateRepo(ctx context.Context, repoURL, clonePath string) error {
    if strings.ContainsAny(repoURL, ";&") {
        return fmt.Errorf("invalid repoURL parameter")
    }

    // Create the parent directory if it doesn't exist
    if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
        return fmt.Errorf("error creating parent directory: %v", err)
    }

    // Perform a shallow clone of the main branch
    cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", "--branch", "main", repoURL, clonePath)

    // Set the GIT_TERMINAL_PROMPT environment variable to prevent interactive prompts
    cloneCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

    // Use a buffer to capture output
    var output bytes.Buffer
    cloneCmd.Stdout = &output
    cloneCmd.Stderr = &output

    // Run the command
    err := cloneCmd.Run()
    if err != nil {
        return fmt.Errorf("error cloning repository: %v, output: %s", err, output.String())
    }

    return nil
}

// Build the application using EAS CLI
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

// Run npm install in the specified package directory
func runNpmInstall(ctx context.Context, packagePath string) error {
	installCmd := exec.CommandContext(ctx, "npm", "install")
	installCmd.Dir = packagePath
	installCmd.Env = os.Environ() // Inherit the environment

	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error running npm install: %v, output: %s", err, string(output))
	}

	return nil
}

// Tail the log file and send updates to the client
func tailLogFile(w http.ResponseWriter, logFilePath string, done chan struct{}) {
	cmd := exec.Command("tail", "-f", logFilePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Failed to get stdout pipe:", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Println("Failed to start tail command:", err)
		return
	}

	reader := bufio.NewReader(stdout)
	for {
		select {
		case <-done:
			cmd.Process.Kill()
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if _, err := w.Write([]byte(line)); err != nil {
				log.Println("Failed to send log message:", err)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// Handle build requests
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

	// Proceed with the build logic
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
	default:
		log.Println("Unsupported platform:", req.Platform)
		http.Error(w, "Unsupported platform", http.StatusBadRequest)
		return
	}

	// Tail the log file
	done := make(chan struct{})
	go tailLogFile(w, "/home/distro/Go/expo-build-service/logs/server.log", done)

	// Build the app
	if err := buildApp(ctx, packagePath, req.Platform, outputFile); err != nil {
		log.Println("Failed to build the app:", err)
		http.Error(w, "Failed to build the app", http.StatusInternalServerError)
		close(done)
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
		close(done)
		return
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		log.Println("Failed to send file to client:", err)
	}

	// Stop tailing the log file
	close(done)
}

// Handle update requests
func updateHandler(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	token := r.Header.Get("Authorization")
	if token != "Bearer your-secret-token" {
		log.Println("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Trigger the update process
	updateMutex.Lock()
	if updateInProgress {
		updateMutex.Unlock()
		log.Println("Update already in progress")
		http.Error(w, "Update already in progress", http.StatusConflict)
		return
	}
	updateInProgress = true
	updateMutex.Unlock()

	done := make(chan struct{})
	go tailLogFile(w, "/home/distro/Go/expo-build-service/logs/server.log", done)

	go func() {
		defer func() {
			updateMutex.Lock()
			updateInProgress = false
			updateMutex.Unlock()
			close(done)
		}()

		// Run the update script
		cmd := exec.Command("/home/distro/Go/expo-build-service/update_server.sh")
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Update failed: %v\nOutput: %s", err, string(output))
		} else {
			log.Println("Update completed successfully.")
		}
	}()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Server update initiated.")
}

// Get the size of a file
func fileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Println("Failed to get file size:", err)
		return 0
	}
	return info.Size()
}

// Health check handler
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Server is up and running.")
}

// Graceful shutdown
func main() {
	// Initialize logging
	initLogging()

	srv := &http.Server{
		Addr: "0.0.0.0:8080",
	}

	// Register your handlers
	http.HandleFunc("/build", authenticate(buildHandler))
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/health", healthHandler)

	// Start the server in a goroutine
	go func() {
		log.Println("Server started at :8080")
		fmt.Println("Server started at :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// Authentication middleware
func authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		expectedToken := os.Getenv("AUTH_TOKEN")
		// Log the tokens for debugging
		log.Printf("Received token: %s", token)
		log.Printf("Expected token: Bearer %s", expectedToken)
		if token != "Bearer "+expectedToken {
			log.Println("Unauthorized access attempt")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// Initialize logging to a file
func initLogging() {
	logDir := "/home/distro/Go/expo-build-service/logs"
	logFile := filepath.Join(logDir, "server.log")

	// Create log directory if it doesn't exist
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create log directory: %v", err)
		}
	}

	// Open log file in append mode, create if not exists
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %v", logFile, err)
	}

	// Set log output to the file
	log.SetOutput(file)

	// Set log flags to include date and time
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
