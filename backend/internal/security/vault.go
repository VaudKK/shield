package security

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// crockfordAlphabet excludes visually ambiguous characters (0/O, 1/I/L) so
// a vault ID or recovery key is easy to transcribe by hand correctly.
const crockfordAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func randomGroups(numGroups, groupSize int) (string, error) {
	groups := make([]string, numGroups)
	for g := range numGroups {
		buf := make([]byte, groupSize)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("generate random bytes: %w", err)
		}
		chars := make([]byte, groupSize)
		for i, b := range buf {
			chars[i] = crockfordAlphabet[int(b)%len(crockfordAlphabet)]
		}
		groups[g] = string(chars)
	}
	return strings.Join(groups, "-"), nil
}

// GenerateVaultID returns a public, non-secret identifier like
// "SH-8F29-KD72". It is safe to display, write down, or say aloud — it is
// not the credential that grants access.
func GenerateVaultID() (string, error) {
	body, err := randomGroups(2, 4)
	if err != nil {
		return "", fmt.Errorf("generate vault id: %w", err)
	}
	return "SH-" + body, nil
}

// GenerateRecoveryKey returns a high-entropy secret like
// "Q7XM-91PK-R4ZT-WL28" (~80 bits of entropy). This is the actual
// credential — treated exactly like a password: hashed with bcrypt before
// storage, never logged, and shown to the user exactly once.
func GenerateRecoveryKey() (string, error) {
	return randomGroups(4, 4)
}
