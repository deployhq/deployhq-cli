// Package telemetry provides anonymous usage tracking for the CLI.
package telemetry

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const telemetryIDFile = "telemetry_id"

// DistinctID returns a persistent anonymous identifier for this machine.
// On first call it generates a UUIDv4 and saves it; subsequent calls
// return the stored value.
func DistinctID(dir string) string {
	path := filepath.Join(dir, telemetryIDFile)

	// Try to load existing ID
	data, err := os.ReadFile(path)
	if err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}

	// Generate new UUIDv4
	id := generateUUID()

	// Persist (best-effort)
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(path, []byte(id+"\n"), 0600)

	return id
}

// HasIdentity returns true if a telemetry ID file exists, meaning the
// user has seen the first-run notice at least once.
func HasIdentity(dir string) bool {
	path := filepath.Join(dir, telemetryIDFile)
	_, err := os.Stat(path)
	return err == nil
}

// generateUUID produces a version-4 UUID using crypto/rand.
func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
