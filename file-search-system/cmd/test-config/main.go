package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/file-search/file-search-system/internal/config"
)

func main() {
	// Try to load configuration
	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("Configuration loaded successfully!")
	fmt.Println("================================")

	// Print key configuration values
	fmt.Printf("Config File: %s\n", cfg.ConfigFile)
	fmt.Printf("Database URL: %s\n", cfg.DatabaseURL)
	fmt.Printf("API Host: %s\n", cfg.APIHost)
	fmt.Printf("API Port: %d\n", cfg.APIPort)
	fmt.Printf("API Workers: %d\n", cfg.APIWorkers)
	fmt.Printf("Index Workers: %d\n", cfg.IndexWorkers)
	fmt.Printf("Watch Paths: %v\n", cfg.WatchPaths)
	fmt.Printf("Ollama Host: %s\n", cfg.OllamaHost)
	fmt.Printf("Ollama Model: %s\n", cfg.OllamaModel)
	fmt.Printf("LLM Model: %s\n", cfg.LLMModel)
	fmt.Printf("Docling Enabled: %v\n", cfg.DoclingEnabled)
	fmt.Printf("Log Level: %s\n", cfg.LogLevel)
	fmt.Printf("Dev Mode: %v\n", cfg.DevMode)

	fmt.Println("\n================================")
	fmt.Println("Full configuration (JSON):")

	// Print full config as JSON
	jsonBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config to JSON: %v", err)
	} else {
		fmt.Println(string(jsonBytes))
	}
}
