package import_manager

import (
	"fmt"
	"regexp"
	"strings"
)

const DefaultMachineID = "default"

var machineIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NormalizeMachineID(machineID string) (string, error) {
	normalized := strings.TrimSpace(machineID)
	if normalized == "" {
		return DefaultMachineID, nil
	}
	if len(normalized) > 64 {
		return "", fmt.Errorf("machine_id must be 64 characters or fewer")
	}
	if !machineIDPattern.MatchString(normalized) {
		return "", fmt.Errorf("machine_id must contain only letters, numbers, hyphens, and underscores")
	}
	return normalized, nil
}

func GenerateScreenshotID(machineID, filename string) string {
	normalized := strings.TrimSpace(machineID)
	if normalized == "" {
		normalized = DefaultMachineID
	}
	return hashStringSHA256(normalized + ":" + filename)
}
