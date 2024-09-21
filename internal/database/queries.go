package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found in DB")

func GetUserByEmail(db *sql.DB, emailAddress string) (id int32, email string, passwordHash string, ok bool, err error) {
	ok = false

	rows, err := db.Query(`SELECT id, email_address, password FROM users WHERE email_address=$1`, emailAddress)
	if err != nil {
		return
	}
	if !rows.Next() {
		return
	}
	defer rows.Close()

	ok = true
	err = rows.Scan(&id, &email, &passwordHash)
	return
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
	rows, err := db.Query(`SELECT id, user_id, location, file_name, file_size, created_at FROM files WHERE id = $1`, file_id)
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

type DBFile struct {
	Id        int32
	UserId    int32
	Location  string
	FileName  string
	FileSize  int64
	CreatedAt time.Time
}
type DBLink struct {
	Id          int32
	AccessKey   string
	AccessCount int64
	FileId      int32
	CreatedBy   int32
	CreatedAt   time.Time
}
