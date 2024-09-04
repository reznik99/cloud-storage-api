package database

import (
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found in DB")

func GetUserByEmail(db *sql.DB, emailAddress string) (id int32, email string, passwordHash string, ok bool, err error) {
	ok = false

	rows, err := db.Query(`SELECT id, email_address, password FROM users WHERE email_address=$1`, emailAddress)
	if err != nil {
		return
	}
	defer rows.Close()

	if !rows.Next() {
		return
	}

	ok = true
	err = rows.Scan(&id, &email, &passwordHash)
	return
}

func UpdateLastSeen(db *sql.DB, id int32) error {
	_, err := db.Exec(`UPDATE users SET last_seen = $1 WHERE id = $2`, time.Now(), id)
	return err
}
