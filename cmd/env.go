// Package cmd implements the CLI commands for diffm.
package cmd

import (
	"os"
	"strconv"
	"strings"
)

// getEnvString retrieves the environment variable for key or returns fallback if unset or empty.
func getEnvString(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

// getEnvInt retrieves the environment variable for key parsed as an integer or returns fallback if unset or invalid.
func getEnvInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

// getEnvBool retrieves the environment variable for key parsed as a boolean or returns fallback if unset or invalid.
// It accepts truthy values ("1", "t", "true", "yes", "y") and falsy values ("0", "f", "false", "no", "n").
func getEnvBool(key string, fallback bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	lower := strings.ToLower(val)
	if lower == "1" || lower == "true" || lower == "t" || lower == "yes" || lower == "y" {
		return true
	}
	if lower == "0" || lower == "false" || lower == "f" || lower == "no" || lower == "n" {
		return false
	}
	return fallback
}
