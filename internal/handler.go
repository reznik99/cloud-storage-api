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
	"gorinidrive.com/api/internal/middleware"
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

	user, err := database.GetUserByEmail(h.Database, req.EmailAddress)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if user == nil { // Email not found
		c.AbortWithError(http.StatusBadRequest, errors.New("incorrect email address or password"))
		return
	}
	match, err := ComparePassword(req.Password, user.Password)
	if err != nil { // Error hashing passwords
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !match { // Password mis-match
		c.AbortWithError(http.StatusBadRequest, errors.New("incorrect email address or password"))
		return
	}

	// Authentication succeeded, update last seen value in database
	if err := database.UpdateLastSeen(h.Database, user.ID); err != nil {
		h.Logger.Warnf("Failed to update last_seen value for user %d: %s", user.ID, err)
	}

	// Set authentication cookie
	h.createCookie(c, user.ID)
	c.JSON(http.StatusOK, LoginRes{
		EmailAddress:      user.EmailAddress,
		CreatedAt:         user.CreatedAt,
		LastSeen:          user.LastSeen,
		WrappedAccountKey: user.WrappedAccountKey,
	})
}

func (h *Handler) Signup(c *gin.Context) {
	var req = &SignupReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	user, err := database.GetUserByEmail(h.Database, req.EmailAddress)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if user != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("email address already taken"))
		return
	}
	// Validate password strength
	if err = ValidatePassword(req.Password); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	// Hash new password
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// Store details
	_, err = h.Database.Exec(`INSERT INTO users(email_address, password, client_random_value, wrapped_account_key) VALUES($1, $2, $3, $4)`,
		req.EmailAddress, passwordHash, req.ClientRandomValue, req.WrappedAccountKey)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// TODO: send email to confirm email ownership

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) Logout(c *gin.Context) {
	// Clear authentication cookie
	h.destroyCookie(c)
	c.JSON(http.StatusOK, nil)
}

func (h *Handler) GetClientRandomValue(c *gin.Context) {
	emailAddress := c.Query("email_address")

	// Get CRV from database for emailAddress
	crv, err := database.GetUserCRVByEmail(h.Database, emailAddress)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// If no CRV found, return default one
	if crv == "" {
		h.Logger.Infof("No CRV found, returning default for user %s", emailAddress)
		crv = os.Getenv("DEFAULT_CRV")
	}

	// Return CRV
	c.JSON(http.StatusOK, &CRVRes{
		ClientRandomValue: crv,
	})
}

func (h *Handler) Session(c *gin.Context) {
	userId := c.Keys["user_id"].(int32)
	user, err := database.GetUserById(h.Database, userId)
	if err != nil || user == nil {
		c.AbortWithError(http.StatusUnauthorized, errors.New("unauthenticated"))
		return
	}

	c.JSON(http.StatusOK, LoginRes{
		EmailAddress:      user.EmailAddress,
		CreatedAt:         user.CreatedAt,
		LastSeen:          user.LastSeen,
		WrappedAccountKey: user.WrappedAccountKey,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req = &ChangePasswordReq{}
	if err := c.BindJSON(req); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// Get user
	user, err := database.GetUserById(h.Database, c.Keys["user_id"].(int32))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid credentials"))
		return
	}
	// Compare old passwords
	match, err := ComparePassword(req.Password, user.Password)
	if err != nil { // Error hashing passwords
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !match { // Password mis-match
		c.AbortWithError(http.StatusForbidden, errors.New("invalid credentials"))
		return
	}
	// Validate password strength
	if err = ValidatePassword(req.Password); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	// Hash new password
	passwordHash, err := HashPassword(req.NewPassword)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// Store new details
	_, err = h.Database.Exec(`UPDATE users SET password=$1, client_random_value=$2, wrapped_account_key=$3 WHERE id=$4`,
		passwordHash, req.NewClientRandomValue, req.NewWrappedAccountKey, user.ID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) DeleteAccount(c *gin.Context) {
	var req = &DeleteAccountReq{}
	if err := c.BindJSON(req); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	// Get user
	user, err := database.GetUserById(h.Database, c.Keys["user_id"].(int32))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid credentials"))
		return
	}
	// Compare passwords
	match, err := ComparePassword(req.Password, user.Password)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !match {
		c.AbortWithError(http.StatusForbidden, errors.New("invalid credentials"))
		return
	}
	// Get files for this account
	rows, err := h.Database.Query(`SELECT id, location FROM files WHERE user_id = $1`, user.ID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()
	// Delete files on disk linked to this account
	for rows.Next() {
		var id int32
		var location string
		if err := rows.Scan(&id, &location); err != nil {
			h.Logger.Errorf("Error scanning row: %s", err)
			continue
		}
		filePath := filepath.Join(h.FileStoragePath, location)
		if err := os.Remove(filePath); err != nil {
			h.Logger.Warnf("Failed to delete file: %s", err)
			continue
		}
	}
	// Delete user (links, files and reset_codes cascade delete)
	_, err = h.Database.Exec(`DELETE FROM users WHERE id=$1`, user.ID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Clear authentication cookie
	h.destroyCookie(c)
	c.JSON(http.StatusOK, nil)
}

func (h *Handler) ListFiles(c *gin.Context) {
	rows, err := h.Database.Query(`SELECT file_name, file_size, created_at, wrapped_file_key FROM files WHERE user_id = $1`, c.Keys["user_id"])
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
		var wrappedFileKey string
		err = rows.Scan(&file_name, &file_size, &added, &wrappedFileKey)
		if err != nil {
			h.Logger.Errorf("Error scanning row: %s", err)
			continue
		}
		output.Files = append(output.Files, File{
			Name:           file_name,
			Size:           file_size,
			Added:          added,
			WrappedFileKey: wrappedFileKey,
		})
	}

	c.JSON(http.StatusOK, output)
}

func (h *Handler) UploadFile(c *gin.Context) {
	wrappedFileKey := c.Request.FormValue("wrapped_file_key")
	if wrappedFileKey == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("encrypted file key is required for file upload"))
		return
	}
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

	// For End-to-End encrypted files http.ServeFile can't detect mime-type so we save it to the database
	fileMimeType := file.Header.Get("Content-Type")

	stmt := `INSERT INTO files(user_id, location, file_name, file_size, file_type, wrapped_file_key) VALUES($1, $2, $3, $4, $5, $6)`
	_, err = h.Database.Exec(stmt, c.Keys["user_id"], location, file.Filename, file.Size, fileMimeType, wrappedFileKey)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err = c.SaveUploadedFile(file, filepath.Join(h.FileStoragePath, location))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	middleware.UploadCount.WithLabelValues(http.StatusText(http.StatusOK)).Add(float64(file.Size))

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) DownloadFile(c *gin.Context) {
	fileName := c.Query("name")
	rows, err := h.Database.Query(`SELECT location, file_type, file_size FROM files WHERE user_id = $1 and file_name = $2`, c.Keys["user_id"], fileName)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !rows.Next() {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}
	defer rows.Close()

	var location string
	var fileSize int64
	var fileMimeType string
	if rows.Scan(&location, &fileMimeType, &fileSize) != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if fileMimeType != "" {
		// For End-to-End encrypted files http.ServeFile can't detect mime-type so we include it from database
		c.Header("Content-Type", fileMimeType)
	}
	middleware.DownloadCount.WithLabelValues(http.StatusText(http.StatusOK)).Add(float64(fileSize))
	// Read file from disk and write to response (handles partial-content/range requests)
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
	defer rows.Close()

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
		h.Logger.Errorf("Failed to delete file: %s", err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) CreateLink(c *gin.Context) {
	var userID = c.Keys["user_id"].(int32)
	var req = &CreateLinkReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	dbFile, found, err := database.GetFileByName(h.Database, userID, req.Name)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	random, err := generateRandomBytes(16)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	accessKey := hex.EncodeToString(random)

	stmt := `INSERT INTO links(access_key, access_count, file_id, created_by) VALUES($1, $2, $3, $4)`
	_, err = h.Database.Exec(stmt, accessKey, 0, dbFile.Id, userID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, &LinkRes{
		AccessKey:   accessKey,
		AccessCount: 0,
		FileId:      dbFile.Id,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		Url:         accessKey,
	})
}

func (h *Handler) GetLink(c *gin.Context) {
	var userID = c.Keys["user_id"].(int32)
	fileName := c.Query("name")

	dbFile, found, err := database.GetFileByName(h.Database, userID, fileName)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	dbLink, found, err := database.GetLinkByFileId(h.Database, userID, dbFile.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("link not found"))
		return
	}

	c.JSON(http.StatusOK, &LinkRes{
		AccessKey:   dbLink.AccessKey,
		AccessCount: dbLink.AccessCount,
		FileId:      dbLink.FileId,
		CreatedBy:   dbLink.CreatedBy,
		CreatedAt:   dbLink.CreatedAt,
	})
}

func (h *Handler) DeleteLink(c *gin.Context) {
	var userID = c.Keys["user_id"].(int32)
	var req = &DeleteLinkReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	dbFile, found, err := database.GetFileByName(h.Database, userID, req.Name)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	stmt := `DELETE FROM links WHERE created_by = $1 AND file_id = $2`
	_, err = h.Database.Exec(stmt, userID, dbFile.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) PreviewLink(c *gin.Context) {
	accessKey := c.Query("access_key")
	rows, err := h.Database.Query(`SELECT file_id FROM links WHERE access_key = $1`, accessKey)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !rows.Next() {
		c.AbortWithError(http.StatusNotFound, errors.New("link not found"))
		return
	}
	defer rows.Close()

	var file_id int32
	if rows.Scan(&file_id) != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	dbFile, found, err := database.GetFileById(h.Database, file_id)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	c.JSON(http.StatusOK, &File{
		Name:  dbFile.FileName,
		Size:  dbFile.FileSize,
		Type:  dbFile.FileType,
		Added: dbFile.CreatedAt,
	})
}

func (h *Handler) DownloadLink(c *gin.Context) {
	accessKey := c.Query("access_key")
	rows, err := h.Database.Query(`SELECT id, file_id FROM links WHERE access_key = $1`, accessKey)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !rows.Next() {
		c.AbortWithError(http.StatusNotFound, errors.New("link not found"))
		return
	}
	defer rows.Close()

	var link_id, file_id int32
	if rows.Scan(&link_id, &file_id) != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	dbFile, found, err := database.GetFileById(h.Database, file_id)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if !found {
		c.AbortWithError(http.StatusNotFound, errors.New("file not found"))
		return
	}

	go func() {
		err := database.UpdateLinkDownloadCount(h.Database, link_id)
		if err != nil {
			h.Logger.Warnf("Failed to update link download count: %s", err)
		}
	}()

	if dbFile.FileType != "" {
		// For End-to-End encrypted files http.ServeFile can't detect mime-type so we include it from database
		c.Header("Content-Type", dbFile.FileType)
	}
	middleware.SharedCount.WithLabelValues(http.StatusText(http.StatusOK)).Add(float64(dbFile.FileSize))
	// Read file from disk and write to response
	c.File(filepath.Join(h.FileStoragePath, dbFile.Location))
}

func (h *Handler) RequestResetPassword(c *gin.Context) {
	// Get user
	emailAddress := c.Query("email_address")
	user, err := database.GetUserByEmail(h.Database, emailAddress)
	if err != nil {
		h.Logger.Errorf("Failed to get user %s from database: %s", emailAddress, err)
		c.AbortWithError(http.StatusInternalServerError, errors.New("failed to get user from database"))
		return
	}
	if user == nil {
		h.Logger.Errorf("Ignoring password reset request for non-existent user %s", emailAddress)
		c.JSON(http.StatusOK, nil)
		return
	}

	// Generate reset-code
	randomBytes, err := generateRandomBytes(16)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	resetCode := hex.EncodeToString(randomBytes)

	// Store reset-code in database
	_, err = h.Database.Exec(`INSERT INTO password_reset_codes(user_id, reset_code) VALUES ($1, $2)`, user.ID, resetCode)
	if err != nil {
		h.Logger.Errorf("Failed to save reset-code into database: %s", err)
		c.AbortWithError(http.StatusUnauthorized, errors.New("failed to save reset-code into database"))
		return
	}

	// Send email with reset-code
	if err := SendPasswordResetEmail(user.EmailAddress, resetCode); err != nil {
		h.Logger.Errorf("Failed to send password reset email: %s", err)
		c.AbortWithError(http.StatusUnauthorized, errors.New("failed to send password reset email"))
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req = &ResetPasswordReq{}
	err := c.BindJSON(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	// Get password reset entry
	dbPR, err := database.GetPasswordResetByCode(h.Database, req.ResetCode)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if dbPR == nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("reset-code invalid"))
		return
	}

	// Delete reset-code (1 time use) regardless of success or failure
	defer func() {
		_, err = h.Database.Exec(`DELETE FROM password_reset_codes WHERE id=$1`, dbPR.Id)
		if err != nil {
			h.Logger.Warnf("Failed to delete reset-code %d after successful use: %s", dbPR.Id, err)
		}
	}()

	if time.Since(dbPR.CreatedAt) > time.Minute*10 {
		c.AbortWithError(http.StatusBadRequest, errors.New("reset-code expired"))
		return
	}
	// Validate password strength
	if err = ValidatePassword(req.NewPassword); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	// Hash new password
	passwordHash, err := HashPassword(req.NewPassword)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// Store new details
	_, err = h.Database.Exec(`UPDATE users SET password=$1, client_random_value=$2, wrapped_account_key=$3 WHERE id=$4`,
		passwordHash, req.NewClientRandomValue, req.NewWrappedAccountKey, dbPR.UserId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// TODO: Delete all files and links in account (as this is a destructive endpoint and file won't be recoverable)
	c.JSON(http.StatusOK, nil)
}
