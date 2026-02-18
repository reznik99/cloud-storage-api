package database

import (
	"database/sql"
	"fmt"
	"time"
)

func GetUserByEmail(db *sql.DB, email_address string) (*DBUser, error) {
	rows, err := db.Query(`SELECT id, email_address, password, created_at, last_seen, wrapped_account_key, allowed_storage FROM users WHERE email_address=$1`, email_address)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, nil
	}
	defer rows.Close()

	user := &DBUser{}
	err = rows.Scan(&user.ID, &user.EmailAddress, &user.Password, &user.CreatedAt, &user.LastSeen, &user.WrappedAccountKey, &user.AllowedStorage)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserById(db *sql.DB, id int32) (*DBUser, error) {
	rows, err := db.Query(`SELECT id, email_address, password, created_at, last_seen, wrapped_account_key, allowed_storage FROM users WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, nil
	}
	defer rows.Close()

	user := &DBUser{}
	err = rows.Scan(&user.ID, &user.EmailAddress, &user.Password, &user.CreatedAt, &user.LastSeen, &user.WrappedAccountKey, &user.AllowedStorage)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserCRVByEmail(db *sql.DB, email_address string) (string, error) {
	rows, err := db.Query(`SELECT client_random_value FROM users WHERE email_address=$1`, email_address)
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", nil
	}
	defer rows.Close()

	crv := ""
	err = rows.Scan(&crv)
	if err != nil {
		return "", err
	}
	return crv, nil
}

func UpdateLastSeen(db *sql.DB, id int32) error {
	_, err := db.Exec(`UPDATE users SET last_seen = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func GetFileByName(db *sql.DB, user_id int32, file_name string) (*DBFile, bool, error) {
	rows, err := db.Query(`SELECT id, user_id, location, file_name, file_size, created_at FROM files WHERE user_id = $1 and file_name = $2`, user_id, file_name)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	defer rows.Close()

	dbFile := &DBFile{}
	err = rows.Scan(&dbFile.Id, &dbFile.UserId, &dbFile.Location, &dbFile.FileName, &dbFile.FileSize, &dbFile.CreatedAt)
	if err != nil {
		return nil, false, err
	}

	return dbFile, true, nil
}

func GetFileById(db *sql.DB, file_id int32) (*DBFile, bool, error) {
	rows, err := db.Query(`SELECT id, user_id, location, file_name, file_size, file_type, created_at FROM files WHERE id = $1`, file_id)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	defer rows.Close()

	dbFile := &DBFile{}
	err = rows.Scan(&dbFile.Id, &dbFile.UserId, &dbFile.Location, &dbFile.FileName, &dbFile.FileSize, &dbFile.FileType, &dbFile.CreatedAt)
	if err != nil {
		return nil, false, err
	}

	return dbFile, true, nil
}

func GetUserStorageMetrics(db *sql.DB, user_id int32) (*DBStorageMetrics, error) {
	// Get size used if any
	var storageUsed int64
	fileRows, err := db.Query(`SELECT COALESCE(SUM(file_size),0) FROM files WHERE user_id = $1`, user_id)
	if err != nil {
		return nil, err
	}
	defer fileRows.Close()
	if fileRows.Next() {
		if err = fileRows.Scan(&storageUsed); err != nil {
			return nil, err
		}
	}

	// Get size allowed for account
	var storageAllowed int64
	userRows, err := db.Query(`SELECT allowed_storage FROM users WHERE id = $1`, user_id)
	if err != nil {
		return nil, err
	}
	defer userRows.Close()
	if userRows.Next() {
		if err = userRows.Scan(&storageAllowed); err != nil {
			return nil, err
		}
	}

	return &DBStorageMetrics{
		UserId:      user_id,
		SizeUsed:    storageUsed,
		SizeAllowed: storageAllowed,
	}, nil
}

func GetLinkByFileId(db *sql.DB, user_id int32, file_id int32) (*DBLink, bool, error) {
	stmt := `SELECT id, access_key, access_count, file_id, created_by, created_at FROM links WHERE created_by=$1 AND file_id=$2`
	rows, err := db.Query(stmt, user_id, file_id)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	defer rows.Close()

	dbLink := &DBLink{}
	err = rows.Scan(&dbLink.Id, &dbLink.AccessKey, &dbLink.AccessCount, &dbLink.FileId, &dbLink.CreatedBy, &dbLink.CreatedAt)
	if err != nil {
		return nil, false, err
	}

	return dbLink, true, nil
}

func UpdateLinkDownloadCount(db *sql.DB, link_id int32) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin err: %s", err)
	}

	rows, err := tx.Query(`SELECT access_count FROM links WHERE id = $1 FOR UPDATE`, link_id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("query err: %s", err)
	}
	if !rows.Next() {
		tx.Rollback()
		return nil
	}

	var accessCount int64
	err = rows.Scan(&accessCount)
	if err != nil {
		rows.Close()
		tx.Rollback()
		return fmt.Errorf("scan err: %s", err)
	}

	rows.Close()
	_, err = tx.Exec(`UPDATE links SET access_count = $1 WHERE id = $2`, accessCount+1, link_id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("exec err: %s", err)
	}

	return tx.Commit()
}

func GetPasswordResetByCode(db *sql.DB, reset_code string) (*DBPasswordReset, error) {
	rows, err := db.Query(`SELECT * FROM password_reset_codes WHERE reset_code = $1`, reset_code)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, nil
	}
	defer rows.Close()

	dbPR := &DBPasswordReset{}
	err = rows.Scan(&dbPR.Id, &dbPR.UserId, &dbPR.ResetCode, &dbPR.CreatedAt)
	if err != nil {
		return nil, err
	}

	return dbPR, nil
}
