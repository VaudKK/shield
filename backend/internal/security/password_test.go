package security

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !VerifyPassword(hash, "correct-horse-battery") {
		t.Error("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("expected incorrect password to fail verification")
	}
}

func TestGenerateToken_UniqueAndHashed(t *testing.T) {
	raw1, hash1, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw2, hash2, err := GenerateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if raw1 == raw2 {
		t.Error("expected distinct raw tokens")
	}
	if hash1 == raw1 {
		t.Error("expected hash to differ from raw token")
	}
	if HashToken(raw1) != hash1 {
		t.Error("expected HashToken(raw) to reproduce the same hash")
	}
	if hash1 == hash2 {
		t.Error("expected distinct hashes for distinct tokens")
	}
}
