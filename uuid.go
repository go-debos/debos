package debos

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ParseGUID enforces canonical RFC 4122 UUID string form:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (uppercase allowed)
// and rejects braces/URN/32-hex etc.
// Returns the parsed UUID as a uuid.UUID.
func ParseGUID(s string) (uuid.UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("incorrect GUID %q: %w", s, err)
	}

	// Canonical form produced by the library (lowercase, hyphenated)
	canonical := u.String()

	// Only accept if the input was already in canonical hyphenated form
	// (case-insensitive). This rejects "{...}", "urn:uuid:..." and 32-hex
	if !strings.EqualFold(s, canonical) {
		return uuid.Nil, fmt.Errorf("incorrect GUID %q: must be canonical form %q", s, canonical)
	}

	return u, nil
}
