package database

import "time"

type User struct {
	ID           int32
	EmailAddress string
	Password     string
	CreatedAt    time.Time
	LastSeen     time.Time
}
