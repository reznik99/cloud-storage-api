package internal

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  error
	}{
		{"valid", "user@example.com", nil},
		{"empty", "", ErrInvalidEmail},
		{"no at sign", "not-an-email", ErrInvalidEmail},
		{"missing local", "@example.com", ErrInvalidEmail},
		{"missing domain", "user@", ErrInvalidEmail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateEmail(tt.email); err != tt.want {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, err, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{"empty", "", ErrPasswordTooShort},
		{"7 chars", "abcdefg", ErrPasswordTooShort},
		{"8 chars weak", "aaaaaaaa", ErrPasswordTooWeak},
		{"strong passphrase", "correct-horse-battery", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePassword(tt.password); err != tt.want {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.want)
			}
		})
	}
}

func TestHashPassword_Roundtrip(t *testing.T) {
	tests := []struct {
		name    string
		compare string
		match   bool
	}{
		{"correct password", "my-secret-password-123", true},
		{"wrong password", "wrong-password", false},
	}

	hash, err := HashPassword("my-secret-password-123")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := ComparePassword(tt.compare, hash)
			if err != nil {
				t.Fatalf("ComparePassword() error: %v", err)
			}
			if match != tt.match {
				t.Errorf("ComparePassword() = %v, want %v", match, tt.match)
			}
		})
	}
}

func TestHashPassword_UniqueSalts(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")

	if h1 == h2 {
		t.Error("HashPassword() produced identical hashes for same password — salt not random")
	}
}

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
	if uint32(len(salt)) != ArgonSaltLength {
		t.Errorf("salt length = %d, want %d", len(salt), ArgonSaltLength)
	}
	if uint32(len(key)) != ArgonKeyLength {
		t.Errorf("key length = %d, want %d", len(key), ArgonKeyLength)
	}
}

func TestDecodeHash_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr error
	}{
		{"empty", "", ErrInvalidHash},
		{"garbage", "not-a-hash", ErrInvalidHash},
		{"too few parts", "$argon2id$v=19$m=65536", ErrInvalidHash},
		{"wrong version", "$argon2id$v=16$m=65536,t=1,p=4$c2FsdHNhbHRzYWx0c2Fs$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaA", ErrIncompatibleVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := decodeHash(tt.hash)
			if err != tt.wantErr {
				t.Errorf("decodeHash() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateTURNCredential(t *testing.T) {
	t.Setenv("TURN_SERVER_SECRET", "test-secret")

	c1, err := GenerateTURNCredential("alice")
	if err != nil {
		t.Fatalf("GenerateTURNCredential() error: %v", err)
	}
	if !strings.Contains(c1.Username, ":alice") {
		t.Errorf("username %q does not contain :alice", c1.Username)
	}
	if c1.Credential == "" {
		t.Error("credential is empty")
	}

	// Different identifier must produce a different credential
	c2, _ := GenerateTURNCredential("bob")
	if c1.Credential == c2.Credential {
		t.Error("different identifiers produced same credential")
	}
}
