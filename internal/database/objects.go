package database

import "time"

type User struct {
	ID           int32
	EmailAddress string
	Password     string
	CreatedAt    time.Time
	LastSeen     time.Time
}

type DBFile struct {
	Id        int32
	UserId    int32
	Location  string
	FileName  string
	FileSize  int64
	FileType  string
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

type DBPasswordReset struct {
	Id        int32
	ResetCode string
	UserId    int32
	CreatedAt time.Time
}
