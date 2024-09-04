package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gorinidrive.com/api/internal"
	"gorinidrive.com/api/internal/database"
	"gorinidrive.com/api/internal/middleware"
)

// Version tag is populated during build
var Version = "Development"
var logger = logrus.New()

func init() {
	logger = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			DisableTimestamp: false,
			FullTimestamp:    true,
			TimestampFormat:  time.DateTime,
		},
		Hooks:        logger.Hooks,
		Level:        logger.GetLevel(),
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
	logger.SetLevel(logrus.InfoLevel)
}

func main() {
	// Enviroment variable
	err := godotenv.Load()
	if err != nil {
		logger.Fatalf("Error loading .env file: %s", err)
	}
	logger.Info("Loaded env variables")

	// Database
	db, err := database.ConnectDB()
	if err != nil {
		logger.Fatalf("Database connection error: %s", err)
	}
	defer db.Close()
	logger.Infof("Connected to %s database", os.Getenv("DB_NAME"))

	// Initialize HTTP server and routes
	logger.Info("Registering middleware...")
	handler := internal.Handler{
		Logger:          logger,
		Database:        db,
		FileStoragePath: os.Getenv("FILE_STORAGE_PATH"),
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.Use(gin.Recovery())
	router.Use(middleware.LogHandler(logger))
	router.Use(middleware.ErrorHandler(logger))
	router.Use(handler.InitCookieStore())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://localhost:8080", "http://localhost:5173"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Register routes
	logger.Info("Registering api routes...")
	router.POST("/api/login", handler.Login)
	router.POST("/api/signup", handler.Signup)
	router.POST("/api/logout", handler.Logout)
	router.GET("/api/files", middleware.Protected(handler.ListFiles))
	router.POST("/api/file", middleware.Protected(handler.UploadFile))
	router.GET("/api/file", middleware.Protected(handler.DownloadFile))
	router.DELETE("/api/file", middleware.Protected(handler.DeleteFile))

	listenAddr := fmt.Sprintf("%s:%s", os.Getenv("LISTEN_ADDR"), os.Getenv("LISTEN_PORT"))
	logger.Infof("Cloud-Storage API (%s) is online '%s'", Version, listenAddr)

	// Listen and serve
	err = router.Run(listenAddr)
	if err != nil {
		logger.Fatalf("Server fatal error: %s", err)
	}
	logger.Info("Server shutdown successfully")
}
