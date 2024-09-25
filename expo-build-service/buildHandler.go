package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer your-secret-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func buildHandler(w http.ResponseWriter, r *http.Request) {
	repoURL := "https://github.com/jdu211171/parents-monolithic.git"
	clonePath := "~/parents-monolithic/parent-notification"

	// Check if the repository exists
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		// Clone the repository
		cloneCmd := exec.Command("git", "clone", repoURL, clonePath)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			log.Printf("Error cloning repository: %s", string(output))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	} else {
		// Reset any local changes
		resetCmd := exec.Command("git", "-C", clonePath, "reset", "--hard")
		if output, err := resetCmd.CombinedOutput(); err != nil {
			log.Printf("Error resetting repository: %s", string(output))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Pull the latest changes
		pullCmd := exec.Command("git", "-C", clonePath, "pull")
		if output, err := pullCmd.CombinedOutput(); err != nil {
			log.Printf("Error pulling repository: %s", string(output))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Remove existing APK files in the clonePath
	apkPattern := filepath.Join(clonePath, "*.apk")
	apkFiles, err := filepath.Glob(apkPattern)
	if err != nil {
		log.Printf("Error finding existing APK files: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for _, apkFile := range apkFiles {
		err := os.Remove(apkFile)
		if err != nil {
			log.Printf("Error removing APK file %s: %v", apkFile, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Build the Expo app
	buildCmd := exec.Command("eas", "build", "--platform", "android", "--local", "--output", "./app.apk")
	buildCmd.Dir = clonePath
	buildCmd.Env = append(os.Environ(), "PATH=/usr/bin/eas")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		log.Printf("Error building project: %s", string(output))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Path to the built APK file
	builtFilePath := filepath.Join(clonePath, "app.apk")

	// Check if the APK file exists
	if _, err := os.Stat(builtFilePath); os.IsNotExist(err) {
		log.Printf("Built APK file not found at %s", builtFilePath)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Send the built file back to the client
	w.Header().Set("Content-Disposition", "attachment; filename=app.apk")
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	http.ServeFile(w, r, builtFilePath)
}

func main() {
	http.HandleFunc("/build", authenticate(buildHandler))
	fmt.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
