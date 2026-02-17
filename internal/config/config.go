package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config holds all sweep configuration options
type Config struct {
	// Global flags
	Yes    bool // Non-interactive mode
	DryRun bool // Show what would be deleted

	// Filters
	OlderThan time.Duration // Only resources older than this
	MinSize   int64         // Only images larger than this (bytes)

	// Type-specific filters
	Dangling   bool // Only dangling images
	NoDangling bool // Exclude dangling images
	Exited     bool // Only exited containers
	Anonymous  bool // Only anonymous volumes
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{}
}

// ParseDuration parses a duration string like "7d", "24h", "1w", "30m"
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	// Try standard Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Parse custom formats: 1d, 2w, 3m (months)
	re := regexp.MustCompile(`^(\d+)([dwmMy])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration: %s (use format like 7d, 2w, 24h)", s)
	}

	n, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case "m", "M":
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %s", unit)
	}
}

// ParseSize parses a size string like "100MB", "1GB", "500KB"
func ParseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	s = strings.ToUpper(strings.TrimSpace(s))

	re := regexp.MustCompile(`^([\d.]+)\s*(B|KB|MB|GB|TB)?$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid size: %s (use format like 100MB, 1GB)", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size value: %s", matches[1])
	}

	unit := matches[2]
	if unit == "" {
		unit = "B"
	}

	var multiplier float64 = 1
	switch unit {
	case "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	}

	return int64(value * multiplier), nil
}
