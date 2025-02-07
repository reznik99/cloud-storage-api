package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"storage.francescogorini.com/api/internal"
	"storage.francescogorini.com/api/internal/database"
	"storage.francescogorini.com/api/internal/middleware"
)

// Version tag is populated during build
var Version = "Development"
var logger = logrus.New()

func init() {
	// Enviroment variable
	err := godotenv.Load()
	if err != nil {
		logger.Fatalf("Error loading .env file: %s", err)
	}

	enviroment := os.Getenv("ENVIROMENT")
	isProduction := strings.EqualFold(enviroment, "Production")

	logger = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			DisableTimestamp: isProduction,
			FullTimestamp:    true,
			TimestampFormat:  time.DateTime,
		},
		Hooks:        logger.Hooks,
		Level:        logrus.InfoLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
	logger.Info("Loaded env variables")
	if isProduction {
		logger.Info("Enviroment 'Production'")
	} else {
		logger.Info("Enviroment 'Development'")
	}
}

func main() {
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
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     internal.CheckWebSocketOrigin,
		},
		WebSockets: sync.Map{},
	}
	gin.SetMode(gin.ReleaseMode)

	middleware.PrometheusInit()
	router := gin.New()
	router.SetTrustedProxies([]string{"127.0.0.1"})
	router.Use(gin.Recovery())
	router.Use(middleware.LogHandler(logger))
	router.Use(middleware.ErrorHandler(logger))
	router.Use(handler.InitCookieStore())
	router.Use(handler.InitCors())

	// Register routes
	logger.Info("Registering protected metrics route...")
	router.GET("/api/metrics", middleware.MetricsHandler())

	logger.Info("Registering api routes...")
	// Auth
	router.POST("/api/login", handler.Login)
	router.POST("/api/signup", handler.Signup)
	router.POST("/api/logout", handler.Logout)
	router.GET("/api/client_random_value", handler.GetClientRandomValue)
	router.GET("/api/session", middleware.Protected(handler.Session))
	router.POST("/api/change_password", middleware.Protected(handler.ChangePassword))
	router.POST("/api/delete_account", middleware.Protected(handler.DeleteAccount))
	// Files
	router.GET("/api/files", middleware.Protected(handler.ListFiles))
	router.POST("/api/file", middleware.Protected(handler.UploadFile))
	router.GET("/api/file", middleware.Protected(handler.DownloadFile))
	router.DELETE("/api/file", middleware.Protected(handler.DeleteFile))
	// File Links
	router.GET("/api/link", middleware.Protected(handler.GetLink))
	router.DELETE("/api/link", middleware.Protected(handler.DeleteLink))
	router.POST("/api/link", middleware.Protected(handler.CreateLink))
	router.GET("/api/link_preview", handler.PreviewLink)
	router.GET("/api/link_download", handler.DownloadLink)
	// Password Reset
	router.GET("/api/reset_password", handler.RequestResetPassword)
	router.POST("/api/reset_password", handler.ResetPassword)
	// Websockets
	router.GET("/ws", handler.NewWebsocket)

	listenAddr := fmt.Sprintf("%s:%s", os.Getenv("LISTEN_ADDR"), os.Getenv("LISTEN_PORT"))
	logger.Infof("Cloud-Storage API (%s) is online '%s'", Version, listenAddr)

	// Start websocket cleanup routine
	go handler.PingSockets()
	// Listen and serve
	err = router.Run(listenAddr)
	if err != nil {
		logger.Fatalf("Server fatal error: %s", err)
	}
	logger.Info("Server shutdown successfully")
}
