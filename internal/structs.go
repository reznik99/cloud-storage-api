package internal

import "time"

// RESTful datatypes

type LoginReq struct {
	EmailAddress string `json:"email_address"`
	Password     string `json:"password"`
}
type LoginRes struct {
	EmailAddress string    `json:"email_address"`
	CreatedAt    time.Time `json:"created_at"`
	LastSeen     time.Time `json:"last_seen"`
}
type CRVRes struct {
	ClientRandomValue string `json:"client_random_value"`
}
type SignupReq struct {
	EmailAddress      string `json:"email_address"`
	Password          string `json:"password"`
	ClientRandomValue string `json:"client_random_value"`
}
type ListFilesRes struct {
	Files []File `json:"files"`
}
type DeleteFileReq struct {
	Name string `json:"name"`
}
type DeleteLinkReq struct {
	Name string `json:"name"`
}
type CreateLinkReq struct {
	Name string `json:"name"`
}
type LinkRes struct {
	AccessKey   string    `json:"access_key"`
	AccessCount int64     `json:"access_count"`
	FileId      int32     `json:"file_id"`
	CreatedBy   int32     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	Url         string    `json:"url"`
}
type ResetPasswordReq struct {
	ResetCode            string `json:"reset_code"`
	NewPassword          string `json:"new_password"`
	NewClientRandomValue string `json:"new_client_random_value"`
}
type ChangePasswordReq struct {
	Password             string `json:"password"`
	NewPassword          string `json:"new_password"`
	NewClientRandomValue string `json:"new_client_random_value"`
}
type DeleteAccountReq struct {
	Password string `json:"password"`
}

// Substructures

type File struct {
	Name  string    `json:"name"`
	Size  int64     `json:"size"`
	Type  string    `json:"type"`
	Added time.Time `json:"added"`
}
