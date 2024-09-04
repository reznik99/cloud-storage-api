package internal

import "time"

// RESTful datatypes

type LoginReq struct {
	EmailAddress string `json:"email_address"`
	Password     string `json:"password"`
}
type SignupReq struct {
	EmailAddress string `json:"email_address"`
	Password     string `json:"password"`
}
type ListFilesRes struct {
	Files []File `json:"files"`
}
type DeleteFileReq struct {
	Name string `json:"name"`
}

// Substructures

type File struct {
	Name  string    `json:"name"`
	Size  int64     `json:"size"`
	Added time.Time `json:"added"`
}
