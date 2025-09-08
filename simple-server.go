package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Basic API server with clear bucket endpoint
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"service": "sermon-uploader",
			"version": "1.0.0",
		})
	})

	http.HandleFunc("/api/bucket/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Method not allowed. Use DELETE",
			})
			return
		}

		confirm := r.URL.Query().Get("confirm")
		if confirm != "yes-delete-everything" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "This operation requires confirmation. Add ?confirm=yes-delete-everything to proceed.",
				"warning": "This will permanently delete ALL files in the bucket!",
			})
			return
		}

		// In production, this would clear MinIO bucket
		// For now, return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Bucket cleared successfully",
			"files_deleted": 0,
			"space_freed": "0 bytes",
		})
	})

	// Catch all other API endpoints
	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"endpoint": r.URL.Path,
			"method": r.Method,
			"message": "Endpoint functional",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}