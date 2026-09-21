package idgen_test

import (
	"regexp"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewUUID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := idgen.NewUUID()
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("NewUUID() = %q, not a valid v4 UUID", id)
		}
		if seen[id] {
			t.Fatalf("NewUUID() returned a duplicate: %q", id)
		}
		seen[id] = true
	}
}
