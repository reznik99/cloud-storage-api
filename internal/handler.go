package internal

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorinidrive.com/api/internal/database"
)

type Handler struct {
	Logger          *logrus.Logger
	Database        *sql.DB
	FileStoragePath string
	cookieDuration  int
}

func (h *Handler) Login(c *gin.Context) {
	var req = &LoginReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	id, _, passwordHash, found, err := database.GetUserByEmail(h.Database, req.EmailAddress)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if !found { // Email not found
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid email/password 1")) // TODO: remove identifier (testing)
		return
	}
	match, err := ComparePassword(req.Password, passwordHash)
	if err != nil { // Error hashing passwords
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !match { // Password mis-match
		c.AbortWithError(http.StatusBadRequest, errors.New("invalid email/password 2")) // TODO: remove identifier (testing)
		return
	}

	// Authentication succeeded, update last seen value in database
	if err := database.UpdateLastSeen(h.Database, id); err != nil {
		h.Logger.Warnf("Failed to update last_seen value for user %d: %s", id, err)
	}

	// Set authentication cookie
	h.createCookie(c, id)
	c.JSON(http.StatusOK, nil)
}

func (h *Handler) Signup(c *gin.Context) {
	var req = &SignupReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, _, _, found, err := database.GetUserByEmail(h.Database, req.EmailAddress)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if found { // email address already taken
		c.AbortWithError(http.StatusBadRequest, errors.New("email address already taken"))
		return
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, err = h.Database.Exec(`INSERT INTO users(email_address, password) VALUES($1, $2)`, req.EmailAddress, passwordHash)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) Logout(c *gin.Context) {
	// Clear authentication cookie
	h.destroyCookie(c)
	c.JSON(http.StatusOK, nil)
}

func (h *Handler) ListFiles(c *gin.Context) {
	rows, err := h.Database.Query(`SELECT file_name, file_size, created_at FROM files WHERE user_id = $1`, c.Keys["user_id"])
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	output := ListFilesRes{
		Files: []File{},
	}

	for rows.Next() {
		var file_name string
		var file_size int64
		var added time.Time
		err = rows.Scan(&file_name, &file_size, &added)
		if err != nil {
			h.Logger.Errorf("Error scanning row: %s", err)
			continue
		}
		output.Files = append(output.Files, File{
			Name:  file_name,
			Size:  file_size,
			Added: added,
		})
	}

	c.JSON(http.StatusOK, output)
}

func (h *Handler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	random, err := generateRandomBytes(32)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	location := hex.EncodeToString(random)

	stmt := `INSERT INTO files(user_id, location, file_name, file_size) VALUES($1, $2, $3, $4)`
	_, err = h.Database.Exec(stmt, c.Keys["user_id"], location, file.Filename, file.Size)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err = c.SaveUploadedFile(file, filepath.Join(h.FileStoragePath, location))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) DownloadFile(c *gin.Context) {
	fileName := c.Query("name")
	rows, err := h.Database.Query(`SELECT location FROM files WHERE user_id = $1 and file_name = $2`, c.Keys["user_id"], fileName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if !rows.Next() {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	var location string
	if rows.Scan(&location) != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// Read file from disk and write to response
	c.File(filepath.Join(h.FileStoragePath, location))
}

func (h *Handler) DeleteFile(c *gin.Context) {
	var req = &DeleteFileReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	rows, err := h.Database.Query(`SELECT id, location FROM files WHERE user_id = $1 and file_name = $2`, c.Keys["user_id"], req.Name)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if !rows.Next() {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	var id int
	var location string
	err = rows.Scan(&id, &location)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, err = h.Database.Exec(`DELETE FROM files WHERE id = $1`, id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	err = os.Remove(filepath.Join(h.FileStoragePath, location))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}
