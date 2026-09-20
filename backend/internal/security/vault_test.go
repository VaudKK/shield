package security

import (
	"regexp"
	"testing"
)

var vaultIDPattern = regexp.MustCompile(`^SH-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}-[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}$`)
var recoveryKeyPattern = regexp.MustCompile(`^([23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}-){3}[23456789ABCDEFGHJKMNPQRSTUVWXYZ]{4}$`)

func TestGenerateVaultID(t *testing.T) {
	id, err := GenerateVaultID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vaultIDPattern.MatchString(id) {
		t.Errorf("vault id %q does not match expected format", id)
	}
}

func TestGenerateVaultID_Unique(t *testing.T) {
	a, _ := GenerateVaultID()
	b, _ := GenerateVaultID()
	if a == b {
		t.Error("expected distinct vault IDs across calls")
	}
}

func TestGenerateRecoveryKey(t *testing.T) {
	key, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recoveryKeyPattern.MatchString(key) {
		t.Errorf("recovery key %q does not match expected format", key)
	}
}

func TestGenerateRecoveryKey_Unique(t *testing.T) {
	a, _ := GenerateRecoveryKey()
	b, _ := GenerateRecoveryKey()
	if a == b {
		t.Error("expected distinct recovery keys across calls")
	}
}

func TestGenerateRecoveryKey_HashableAndVerifiable(t *testing.T) {
	key, err := GenerateRecoveryKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hash, err := HashPassword(key)
	if err != nil {
		t.Fatalf("hash recovery key: %v", err)
	}
	if !VerifyPassword(hash, key) {
		t.Error("expected recovery key to verify against its own hash")
	}
	if VerifyPassword(hash, "wrong-key") {
		t.Error("expected a different value not to verify")
	}
}
