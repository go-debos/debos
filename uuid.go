package debos

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ParseGUID enforces canonical RFC 4122 UUID string form:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (uppercase allowed)
// and rejects braces/URN/32-hex etc.
func ParseGUID(s string) (string, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return "", fmt.Errorf("incorrect GUID %q: %w", s, err)
	}

	// Canonical form produced by the library (lowercase, hyphenated)
	canonical := u.String()

	// Only accept if the input was already in canonical hyphenated form
	// (case-insensitive). This rejects "{...}", "urn:uuid:..." and 32-hex
	if !strings.EqualFold(s, canonical) {
		return "", fmt.Errorf("incorrect GUID %q: must be canonical form %q", s, canonical)
	}

	return canonical, nil
}

// UUID5 returns a deterministic RFC 4122 UUIDv5 for (namespace, data).
// The namespace must be a canonical UUID string (e.g. "6ba7b810-9dad-11d1-80b4-00c04fd430c8").
// Panics if the namespace is not a valid UUID.
func GenerateUUID5(namespace string, data string) string {
	id := uuid.NewSHA1(uuid.MustParse(namespace), []byte(data))
	return id.String()
}
