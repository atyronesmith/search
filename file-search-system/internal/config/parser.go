package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Parser handles parsing of .cfg files
type Parser struct {
	sections map[string]map[string]string
}

// NewParser creates a new config parser
func NewParser() *Parser {
	return &Parser{
		sections: make(map[string]map[string]string),
	}
}

// LoadFile loads and parses a configuration file
func (p *Parser) LoadFile(filename string) error {
	// Expand home directory if present
	if strings.HasPrefix(filename, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %v", err)
		}
		filename = filepath.Join(home, filename[2:])
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %v", filename, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Ignore close error - file was already read
			_ = err
		}
	}()

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			if p.sections[currentSection] == nil {
				p.sections[currentSection] = make(map[string]string)
			}
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		if currentSection == "" {
			// Global section
			if p.sections[""] == nil {
				p.sections[""] = make(map[string]string)
			}
			p.sections[""][key] = value
		} else {
			p.sections[currentSection][key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	return nil
}

// Merge merges another parser's data into this one
// Values from the other parser override existing values
func (p *Parser) Merge(other *Parser) {
	for section, values := range other.sections {
		if p.sections[section] == nil {
			p.sections[section] = make(map[string]string)
		}
		for key, value := range values {
			p.sections[section][key] = value
		}
	}
}

// GetString retrieves a string value from the config
func (p *Parser) GetString(section, key, defaultValue string) string {
	if sectionData, ok := p.sections[section]; ok {
		if value, ok := sectionData[key]; ok {
			return value
		}
	}
	return defaultValue
}

// GetInt retrieves an integer value from the config
func (p *Parser) GetInt(section, key string, defaultValue int) int {
	str := p.GetString(section, key, "")
	if str == "" {
		return defaultValue
	}
	if val, err := strconv.Atoi(str); err == nil {
		return val
	}
	return defaultValue
}

// GetFloat retrieves a float value from the config
func (p *Parser) GetFloat(section, key string, defaultValue float64) float64 {
	str := p.GetString(section, key, "")
	if str == "" {
		return defaultValue
	}
	if val, err := strconv.ParseFloat(str, 64); err == nil {
		return val
	}
	return defaultValue
}

// GetBool retrieves a boolean value from the config
func (p *Parser) GetBool(section, key string, defaultValue bool) bool {
	str := p.GetString(section, key, "")
	if str == "" {
		return defaultValue
	}
	str = strings.ToLower(str)
	if str == "true" || str == "yes" || str == "on" || str == "1" {
		return true
	}
	if str == "false" || str == "no" || str == "off" || str == "0" {
		return false
	}
	return defaultValue
}

// GetDuration retrieves a duration value from the config
func (p *Parser) GetDuration(section, key string, defaultValue time.Duration) time.Duration {
	str := p.GetString(section, key, "")
	if str == "" {
		return defaultValue
	}
	if duration, err := time.ParseDuration(str); err == nil {
		return duration
	}
	// Try parsing as seconds
	if seconds, err := strconv.Atoi(str); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return defaultValue
}

// GetStringSlice retrieves a slice of strings from the config
func (p *Parser) GetStringSlice(section, key string, defaultValue []string) []string {
	str := p.GetString(section, key, "")
	if str == "" {
		return defaultValue
	}

	// Split by comma and trim spaces
	parts := strings.Split(str, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			// Expand home directory in paths
			if strings.HasPrefix(trimmed, "~/") {
				home, _ := os.UserHomeDir()
				trimmed = filepath.Join(home, trimmed[2:])
			}
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}
