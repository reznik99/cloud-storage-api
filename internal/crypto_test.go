package internal

import (
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// --- ValidateEmail ---

func TestValidateEmail_Valid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"user+tag@example.com",
		"first.last@example.com",
		"user@subdomain.example.com",
	}
	for _, email := range valid {
		if err := ValidateEmail(email); err != nil {
			t.Errorf("ValidateEmail(%q) returned error: %v", email, err)
		}
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"not-an-email",
		"@missing-local.com",
		"missing-domain@",
		"spaces in@address.com",
	}
	for _, email := range invalid {
		if err := ValidateEmail(email); err != ErrInvalidEmail {
			t.Errorf("ValidateEmail(%q) = %v, want ErrInvalidEmail", email, err)
		}
	}
}

// --- ValidatePassword ---

func TestValidatePassword_TooShort(t *testing.T) {
	if err := ValidatePassword("abc"); err != ErrPasswordTooShort {
		t.Errorf("ValidatePassword(short) = %v, want ErrPasswordTooShort", err)
	}
}

func TestValidatePassword_TooWeak(t *testing.T) {
	// 8+ chars but trivially guessable
	if err := ValidatePassword("aaaaaaaa"); err != ErrPasswordTooWeak {
		t.Errorf("ValidatePassword(weak) = %v, want ErrPasswordTooWeak", err)
	}
}

func TestValidatePassword_Strong(t *testing.T) {
	if err := ValidatePassword("correct-horse-battery"); err != nil {
		t.Errorf("ValidatePassword(strong) = %v, want nil", err)
	}
}

func TestValidatePassword_ExactMinLength(t *testing.T) {
	// Exactly 8 chars but weak
	if err := ValidatePassword("12345678"); err != ErrPasswordTooWeak {
		t.Errorf("ValidatePassword(8-char-weak) = %v, want ErrPasswordTooWeak", err)
	}
}

// --- HashPassword / ComparePassword ---

func TestHashPassword_Roundtrip(t *testing.T) {
	password := "my-secret-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	match, err := ComparePassword(password, hash)
	if err != nil {
		t.Fatalf("ComparePassword() error: %v", err)
	}
	if !match {
		t.Error("ComparePassword() returned false for correct password")
	}
}

func TestHashPassword_WrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	match, err := ComparePassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("ComparePassword() error: %v", err)
	}
	if match {
		t.Error("ComparePassword() returned true for wrong password")
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")

	if h1 == h2 {
		t.Error("HashPassword() produced identical hashes for same password — salt not random")
	}
}

func TestHashPassword_Format(t *testing.T) {
	hash, err := HashPassword("test-password")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash does not start with $argon2id$: %s", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash has %d parts, want 6: %s", len(parts), hash)
	}
}

// --- decodeHash ---

func TestDecodeHash_Valid(t *testing.T) {
	hash, _ := HashPassword("test")

	params, salt, key, err := decodeHash(hash)
	if err != nil {
		t.Fatalf("decodeHash() error: %v", err)
	}

	if params.memory != ArgonMemory {
		t.Errorf("memory = %d, want %d", params.memory, ArgonMemory)
	}
	if params.iterations != ArgonTime {
		t.Errorf("iterations = %d, want %d", params.iterations, ArgonTime)
	}
	if params.parallelism != ArgonThreads {
		t.Errorf("parallelism = %d, want %d", params.parallelism, ArgonThreads)
	}
	if len(salt) != int(ArgonSaltLength) {
		t.Errorf("salt length = %d, want %d", len(salt), ArgonSaltLength)
	}
	if len(key) != int(ArgonKeyLength) {
		t.Errorf("key length = %d, want %d", len(key), ArgonKeyLength)
	}
}

func TestDecodeHash_InvalidFormat(t *testing.T) {
	cases := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"garbage", "not-a-hash"},
		{"too few parts", "$argon2id$v=19$m=65536"},
		{"bad version", "$argon2id$v=99$m=65536,t=1,p=4$c2FsdA$aGFzaA"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := decodeHash(tc.hash)
			if err == nil {
				t.Error("decodeHash() expected error, got nil")
			}
		})
	}
}

func TestDecodeHash_IncompatibleVersion(t *testing.T) {
	// Craft a hash with a wrong argon2 version
	hash := "$argon2id$v=16$m=65536,t=1,p=4$c2FsdHNhbHRzYWx0c2Fs$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaA"
	_, _, _, err := decodeHash(hash)
	if err != ErrIncompatibleVersion {
		t.Errorf("decodeHash(wrong version) = %v, want ErrIncompatibleVersion", err)
	}
}

// --- GenerateTURNCredential ---

func TestGenerateTURNCredential_Format(t *testing.T) {
	t.Setenv("TURN_SERVER_SECRET", "test-secret")

	cred, err := GenerateTURNCredential("user123")
	if err != nil {
		t.Fatalf("GenerateTURNCredential() error: %v", err)
	}

	if !strings.Contains(cred.Username, ":user123") {
		t.Errorf("username %q does not contain :user123", cred.Username)
	}

	if cred.Credential == "" {
		t.Error("credential is empty")
	}
}

func TestGenerateTURNCredential_DifferentIdentifiers(t *testing.T) {
	t.Setenv("TURN_SERVER_SECRET", "test-secret")

	c1, _ := GenerateTURNCredential("alice")
	c2, _ := GenerateTURNCredential("bob")

	if c1.Credential == c2.Credential {
		t.Error("different identifiers produced same credential")
	}
}

func TestGenerateTURNCredential_DeterministicForSameInput(t *testing.T) {
	t.Setenv("TURN_SERVER_SECRET", "test-secret")

	// Same identifier at same second should produce same credential.
	// We call twice rapidly — the username includes a unix timestamp,
	// so as long as both calls happen in the same second, they match.
	c1, _ := GenerateTURNCredential("alice")
	c2, _ := GenerateTURNCredential("alice")

	if c1.Username == c2.Username && c1.Credential != c2.Credential {
		t.Error("same username produced different credentials")
	}
}

// --- Argon2 parameter constants ---

func TestArgonConstants(t *testing.T) {
	if argon2.Version != 0x13 {
		t.Fatalf("unexpected argon2 version: %d", argon2.Version)
	}
	if ArgonMemory != 64*1024 {
		t.Error("ArgonMemory changed from 64 MiB")
	}
	if ArgonKeyLength != 32 {
		t.Error("ArgonKeyLength changed from 32 bytes")
	}
}
