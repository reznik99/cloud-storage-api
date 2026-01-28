package internal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nbutton23/zxcvbn-go"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
	ErrPasswordMismatch    = errors.New("passwords don't match")
	ErrPasswordTooShort    = errors.New("password is shorter than 8 characters")
	ErrPasswordTooWeak     = errors.New("password is too weak")
)

const (
	ArgonTime         = 1
	ArgonMemory       = 64 * 1024
	ArgonThreads      = 4
	ArgonSaltLength   = 16
	ArgonKeyLength    = 32
	MinPasswordLength = 8
	MinPasswordScore  = 1
)

type ArgonParams struct {
	memory      uint32 // ArgonMemory
	iterations  uint32 // ArgonTime
	parallelism uint8  // ArgonThreads
	saltLength  uint32 // ArgonSaltLength
	keyLength   uint32 // ArgonKeyLength
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	res := zxcvbn.PasswordStrength(password, nil)
	if res.Score < MinPasswordScore {
		return ErrPasswordTooWeak
	}
	return nil
}

func ComparePassword(password string, encodedHash string) (bool, error) {
	// Decode existing password hash for parameter information (future proofing in case we change the defaults)
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	otherHash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)

	// Check that the contents of the hashed passwords are identical. Note
	// that we are using the subtle.ConstantTimeCompare() function for this
	// to help prevent timing attacks.
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func HashPassword(password string) (string, error) {
	salt, err := generateRandomBytes(ArgonSaltLength)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, ArgonTime, ArgonMemory, ArgonThreads, ArgonKeyLength)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Return a string using the standard encoded hash representation.
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, ArgonMemory, ArgonTime, ArgonThreads, b64Salt, b64Hash)

	return encodedHash, nil
}

func generateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func decodeHash(encodedHash string) (p *ArgonParams, salt, hash []byte, err error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	p = &ArgonParams{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	p.saltLength = uint32(len(salt))

	hash, err = base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	p.keyLength = uint32(len(hash))

	return p, salt, hash, nil
}

type TURNCredential struct {
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

func GenerateTURNCredential(identifier string) (TURNCredential, error) {
	secret := os.Getenv("TURN_SERVER_SECRET")
	toBeSigned := fmt.Sprintf("%d%d:%s", time.Now().Unix(), 3600, identifier)
	hm := hmac.New(sha1.New, []byte(secret))

	credential := hm.Sum([]byte(toBeSigned))

	return TURNCredential{
		Username:   identifier,
		Credential: base64.StdEncoding.EncodeToString(credential),
	}, nil
}
