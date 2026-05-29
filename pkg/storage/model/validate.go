package model

import (
	"fmt"
	"regexp"
)

// validScanIDRegex matches only alphanumeric characters, hyphens, and underscores.
var validScanIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateScanID validates that the scanID only contains safe characters and
// does not contain path traversal sequences. It is shared across storage
// backends so the validation rules stay consistent.
func ValidateScanID(scanID string) error {
	if scanID == "" {
		return fmt.Errorf("scanID cannot be empty")
	}
	if !validScanIDRegex.MatchString(scanID) {
		return fmt.Errorf("scanID contains invalid characters: only alphanumeric characters, hyphens, and underscores are allowed")
	}
	return nil
}
