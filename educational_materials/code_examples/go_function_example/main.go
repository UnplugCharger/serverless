package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

// Response represents the function output structure
type Response struct {
	Message     string            `json:"message"`
	Timestamp   string            `json:"timestamp"`
	Environment map[string]string `json:"environment"`
}

func main() {
	// Get input from environment variables
	name := os.Getenv("NAME")
	if name == "" {
		name = "World"
	}

	// Process the input
	message := fmt.Sprintf("Hello, %s!", name)

	// Create a response object
	response := Response{
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
		Environment: map[string]string{
			"go_version": runtime.Version(),
			"platform":   runtime.GOOS,
		},
	}

	// Convert to JSON and print
	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Printf("Error creating JSON response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonResponse))
}
