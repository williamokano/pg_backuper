package backup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetPgpassPath returns the path to the .pgpass file
// Priority: 1) Config specified path, 2) /config/.pgpass (Docker), 3) ~/.pgpass (standard)
func GetPgpassPath(configPath string) (string, error) {
	// If explicitly configured, use that
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("configured pgpass file not found: %s", configPath)
	}

	// Try Docker default location
	dockerPath := "/config/.pgpass"
	if _, err := os.Stat(dockerPath); err == nil {
		return dockerPath, nil
	}

	// Try standard location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	standardPath := filepath.Join(homeDir, ".pgpass")
	if _, err := os.Stat(standardPath); err == nil {
		return standardPath, nil
	}

	return "", fmt.Errorf("no .pgpass file found (tried: %s, %s, %s)", configPath, dockerPath, standardPath)
}

// ValidatePgpassPermissions checks that .pgpass has correct permissions (0600)
func ValidatePgpassPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat .pgpass file: %w", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		return fmt.Errorf(".pgpass file has incorrect permissions %o, must be 0600 (readable/writable by owner only)", mode)
	}

	return nil
}

// VerifyPgpassEntry checks if a .pgpass file contains an entry for the given connection
// This is mainly for validation/debugging purposes
func VerifyPgpassEntry(pgpassPath, host, port, database, username string) (bool, error) {
	file, err := os.Open(pgpassPath)
	if err != nil {
		return false, fmt.Errorf("failed to open .pgpass file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse line: hostname:port:database:username:password
		parts := strings.Split(line, ":")
		if len(parts) != 5 {
			continue
		}

		// Check if entry matches (with wildcard support)
		if matchField(parts[0], host) &&
			matchField(parts[1], port) &&
			matchField(parts[2], database) &&
			matchField(parts[3], username) {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading .pgpass file: %w", err)
	}

	return false, nil
}

// matchField checks if a pattern matches a value (supports * wildcard)
func matchField(pattern, value string) bool {
	return pattern == "*" || pattern == value
}
