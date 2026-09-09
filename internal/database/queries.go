package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func GetUserByEmail(ctx context.Context, db *sql.DB, email_address string) (*DBUser, error) {
	user := &DBUser{}
	err := db.QueryRowContext(ctx, `SELECT id, email_address, password, created_at, last_seen, wrapped_account_key, allowed_storage FROM users WHERE email_address=$1`, email_address).
		Scan(&user.ID, &user.EmailAddress, &user.Password, &user.CreatedAt, &user.LastSeen, &user.WrappedAccountKey, &user.AllowedStorage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserById(ctx context.Context, db *sql.DB, id int32) (*DBUser, error) {
	user := &DBUser{}
	err := db.QueryRowContext(ctx, `SELECT id, email_address, password, created_at, last_seen, wrapped_account_key, allowed_storage FROM users WHERE id=$1`, id).
		Scan(&user.ID, &user.EmailAddress, &user.Password, &user.CreatedAt, &user.LastSeen, &user.WrappedAccountKey, &user.AllowedStorage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserCRVByEmail(ctx context.Context, db *sql.DB, email_address string) (string, error) {
	crv := ""
	err := db.QueryRowContext(ctx, `SELECT client_random_value FROM users WHERE email_address=$1`, email_address).Scan(&crv)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return crv, nil
}

func UpdateLastSeen(ctx context.Context, db *sql.DB, id int32) error {
	_, err := db.ExecContext(ctx, `UPDATE users SET last_seen = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func GetFileByName(ctx context.Context, db *sql.DB, user_id int32, file_name string) (*DBFile, bool, error) {
	dbFile := &DBFile{}
	err := db.QueryRowContext(ctx, `SELECT id, user_id, location, file_name, file_size, created_at FROM files WHERE user_id = $1 and file_name = $2`, user_id, file_name).
		Scan(&dbFile.Id, &dbFile.UserId, &dbFile.Location, &dbFile.FileName, &dbFile.FileSize, &dbFile.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return dbFile, true, nil
}

func GetFileById(ctx context.Context, db *sql.DB, file_id int32) (*DBFile, bool, error) {
	dbFile := &DBFile{}
	err := db.QueryRowContext(ctx, `SELECT id, user_id, location, file_name, file_size, file_type, created_at FROM files WHERE id = $1`, file_id).
		Scan(&dbFile.Id, &dbFile.UserId, &dbFile.Location, &dbFile.FileName, &dbFile.FileSize, &dbFile.FileType, &dbFile.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return dbFile, true, nil
}

func GetUserStorageMetrics(ctx context.Context, db *sql.DB, user_id int32) (*DBStorageMetrics, error) {
	// Get size used if any. No row leaves the zero value in place, as before.
	var storageUsed int64
	err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(file_size),0) FROM files WHERE user_id = $1`, user_id).Scan(&storageUsed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Get size allowed for account
	var storageAllowed int64
	err = db.QueryRowContext(ctx, `SELECT allowed_storage FROM users WHERE id = $1`, user_id).Scan(&storageAllowed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return &DBStorageMetrics{
		UserId:      user_id,
		SizeUsed:    storageUsed,
		SizeAllowed: storageAllowed,
	}, nil
}

func GetLinkByFileId(ctx context.Context, db *sql.DB, user_id int32, file_id int32) (*DBLink, bool, error) {
	stmt := `SELECT id, access_key, access_count, file_id, created_by, created_at FROM links WHERE created_by=$1 AND file_id=$2`
	dbLink := &DBLink{}
	err := db.QueryRowContext(ctx, stmt, user_id, file_id).
		Scan(&dbLink.Id, &dbLink.AccessKey, &dbLink.AccessCount, &dbLink.FileId, &dbLink.CreatedBy, &dbLink.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return dbLink, true, nil
}

func UpdateLinkDownloadCount(ctx context.Context, db *sql.DB, link_id int32) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin err: %s", err)
	}
	defer func() { _ = tx.Rollback() }()

	var accessCount int64
	err = tx.QueryRowContext(ctx, `SELECT access_count FROM links WHERE id = $1 FOR UPDATE`, link_id).Scan(&accessCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query err: %s", err)
	}

	_, err = tx.ExecContext(ctx, `UPDATE links SET access_count = $1 WHERE id = $2`, accessCount+1, link_id)
	if err != nil {
		return fmt.Errorf("exec err: %s", err)
	}

	return tx.Commit()
}

func GetPasswordResetByCode(ctx context.Context, db *sql.DB, reset_code string) (*DBPasswordReset, error) {
	dbPR := &DBPasswordReset{}
	err := db.QueryRowContext(ctx, `SELECT * FROM password_reset_codes WHERE reset_code = $1`, reset_code).
		Scan(&dbPR.Id, &dbPR.UserId, &dbPR.ResetCode, &dbPR.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return dbPR, nil
}
