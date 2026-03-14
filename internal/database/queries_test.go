//go:build integration

package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "gdrive",
			"POSTGRES_PASSWORD": "test",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		panic("failed to start postgres container: " + err.Error())
	}

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		panic("failed to open db: " + err.Error())
	}

	schema, err := os.ReadFile("../../schema/new.sql")
	if err != nil {
		panic("failed to read schema: " + err.Error())
	}
	if _, err := testDB.Exec(string(schema)); err != nil {
		panic("failed to apply schema: " + err.Error())
	}

	code := m.Run()

	pg.Terminate(ctx)
	os.Exit(code)
}

// truncateAll clears all tables between tests.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec("TRUNCATE users, files, links, password_reset_codes RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// seedUser inserts a user and returns its ID.
func seedUser(t *testing.T, email string) int32 {
	t.Helper()
	var id int32
	err := testDB.QueryRow(
		`INSERT INTO users (email_address, password, client_random_value, wrapped_account_key)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		email, "hashed-pw", "test-crv", "test-wak",
	).Scan(&id)
	if err != nil {
		t.Fatalf("seedUser: %v", err)
	}
	return id
}

// seedFile inserts a file for a user and returns its ID.
func seedFile(t *testing.T, userID int32, fileName string, size int64) int32 {
	t.Helper()
	var id int32
	err := testDB.QueryRow(
		`INSERT INTO files (user_id, location, wrapped_file_key, file_name, file_size, file_type)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, "abc123", "wrapped-key", fileName, size, "application/octet-stream",
	).Scan(&id)
	if err != nil {
		t.Fatalf("seedFile: %v", err)
	}
	return id
}

// seedLink inserts a link for a file and returns its ID.
func seedLink(t *testing.T, fileID, userID int32, accessKey string) int32 {
	t.Helper()
	var id int32
	err := testDB.QueryRow(
		`INSERT INTO links (access_key, access_count, file_id, created_by)
		 VALUES ($1, 0, $2, $3) RETURNING id`,
		accessKey, fileID, userID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seedLink: %v", err)
	}
	return id
}

// --- GetUserByEmail / GetUserById ---

func TestGetUserByEmail(t *testing.T) {
	truncateAll(t)
	seedUser(t, "alice@test.com")

	t.Run("found", func(t *testing.T) {
		user, err := GetUserByEmail(testDB, "alice@test.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user == nil {
			t.Fatal("expected user, got nil")
		}
		if user.EmailAddress != "alice@test.com" {
			t.Errorf("email = %q, want alice@test.com", user.EmailAddress)
		}
		if user.AllowedStorage != 1024000000 {
			t.Errorf("allowed_storage = %d, want 1024000000", user.AllowedStorage)
		}
	})

	t.Run("not found", func(t *testing.T) {
		user, err := GetUserByEmail(testDB, "nobody@test.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user != nil {
			t.Error("expected nil for non-existent user")
		}
	})
}

func TestGetUserById(t *testing.T) {
	truncateAll(t)
	id := seedUser(t, "bob@test.com")

	t.Run("found", func(t *testing.T) {
		user, err := GetUserById(testDB, id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user == nil {
			t.Fatal("expected user, got nil")
		}
		if user.EmailAddress != "bob@test.com" {
			t.Errorf("email = %q, want bob@test.com", user.EmailAddress)
		}
	})

	t.Run("not found", func(t *testing.T) {
		user, err := GetUserById(testDB, 99999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user != nil {
			t.Error("expected nil for non-existent user")
		}
	})
}

// --- GetUserCRVByEmail ---

func TestGetUserCRVByEmail(t *testing.T) {
	truncateAll(t)
	seedUser(t, "crv@test.com")

	t.Run("found", func(t *testing.T) {
		crv, err := GetUserCRVByEmail(testDB, "crv@test.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if crv != "test-crv" {
			t.Errorf("crv = %q, want test-crv", crv)
		}
	})

	t.Run("not found", func(t *testing.T) {
		crv, err := GetUserCRVByEmail(testDB, "nobody@test.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if crv != "" {
			t.Errorf("crv = %q, want empty string", crv)
		}
	})
}

// --- UpdateLastSeen ---

func TestUpdateLastSeen(t *testing.T) {
	truncateAll(t)
	id := seedUser(t, "lastseen@test.com")

	before, _ := GetUserById(testDB, id)
	time.Sleep(10 * time.Millisecond)

	if err := UpdateLastSeen(testDB, id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, _ := GetUserById(testDB, id)
	if !after.LastSeen.After(before.LastSeen) {
		t.Error("last_seen was not updated")
	}
}

// --- GetFileByName / GetFileById ---

func TestGetFileByName(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "files@test.com")
	seedFile(t, uid, "doc.enc", 1024)

	t.Run("found", func(t *testing.T) {
		file, found, err := GetFileByName(testDB, uid, "doc.enc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || file == nil {
			t.Fatal("expected file, got not found")
		}
		if file.FileName != "doc.enc" {
			t.Errorf("file_name = %q, want doc.enc", file.FileName)
		}
		if file.FileSize != 1024 {
			t.Errorf("file_size = %d, want 1024", file.FileSize)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := GetFileByName(testDB, uid, "nope.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
	})

	t.Run("wrong user", func(t *testing.T) {
		_, found, err := GetFileByName(testDB, 99999, "doc.enc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found for wrong user")
		}
	})
}

func TestGetFileById(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "files@test.com")
	fid := seedFile(t, uid, "photo.enc", 2048)

	t.Run("found", func(t *testing.T) {
		file, found, err := GetFileById(testDB, fid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || file == nil {
			t.Fatal("expected file, got not found")
		}
		if file.FileType != "application/octet-stream" {
			t.Errorf("file_type = %q, want application/octet-stream", file.FileType)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := GetFileById(testDB, 99999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
	})
}

// --- GetUserStorageMetrics ---

func TestGetUserStorageMetrics(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "storage@test.com")

	t.Run("no files", func(t *testing.T) {
		m, err := GetUserStorageMetrics(testDB, uid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.SizeUsed != 0 {
			t.Errorf("size_used = %d, want 0", m.SizeUsed)
		}
		if m.SizeAllowed != 1024000000 {
			t.Errorf("size_allowed = %d, want 1024000000", m.SizeAllowed)
		}
	})

	t.Run("with files", func(t *testing.T) {
		seedFile(t, uid, "a.enc", 100)
		seedFile(t, uid, "b.enc", 250)

		m, err := GetUserStorageMetrics(testDB, uid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.SizeUsed != 350 {
			t.Errorf("size_used = %d, want 350", m.SizeUsed)
		}
	})
}

// --- GetLinkByFileId ---

func TestGetLinkByFileId(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "links@test.com")
	fid := seedFile(t, uid, "shared.enc", 512)
	seedLink(t, fid, uid, "abc-key")

	t.Run("found", func(t *testing.T) {
		link, found, err := GetLinkByFileId(testDB, uid, fid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !found || link == nil {
			t.Fatal("expected link, got not found")
		}
		if link.AccessKey != "abc-key" {
			t.Errorf("access_key = %q, want abc-key", link.AccessKey)
		}
		if link.AccessCount != 0 {
			t.Errorf("access_count = %d, want 0", link.AccessCount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found, err := GetLinkByFileId(testDB, uid, 99999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
	})
}

// --- UpdateLinkDownloadCount ---

func TestUpdateLinkDownloadCount(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "dl@test.com")
	fid := seedFile(t, uid, "dl.enc", 100)
	lid := seedLink(t, fid, uid, "dl-key")

	// Increment twice
	for range 2 {
		if err := UpdateLinkDownloadCount(testDB, lid); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	link, _, _ := GetLinkByFileId(testDB, uid, fid)
	if link.AccessCount != 2 {
		t.Errorf("access_count = %d, want 2", link.AccessCount)
	}
}

// --- GetPasswordResetByCode ---

func TestGetPasswordResetByCode(t *testing.T) {
	truncateAll(t)
	uid := seedUser(t, "reset@test.com")
	testDB.Exec(`INSERT INTO password_reset_codes (user_id, reset_code) VALUES ($1, $2)`, uid, "reset-abc")

	t.Run("found", func(t *testing.T) {
		pr, err := GetPasswordResetByCode(testDB, "reset-abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil {
			t.Fatal("expected reset code, got nil")
		}
		if pr.ResetCode != "reset-abc" {
			t.Errorf("reset_code = %q, want reset-abc", pr.ResetCode)
		}
		if pr.UserId != uid {
			t.Errorf("user_id = %d, want %d", pr.UserId, uid)
		}
	})

	t.Run("not found", func(t *testing.T) {
		pr, err := GetPasswordResetByCode(testDB, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr != nil {
			t.Error("expected nil for non-existent code")
		}
	})
}
