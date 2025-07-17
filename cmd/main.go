// main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/X3nonxe/gopsy-backend/internal/config"
	availabilityHandler "github.com/X3nonxe/gopsy-backend/internal/delivery/http/handler"
	userHandler "github.com/X3nonxe/gopsy-backend/internal/delivery/http/handler"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/handler"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/middleware"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/router"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	userRepo "github.com/X3nonxe/gopsy-backend/internal/repository"
	userUsecase "github.com/X3nonxe/gopsy-backend/internal/usecase"
	"github.com/X3nonxe/gopsy-backend/internal/usecase"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Setup logger
	setupLogger(cfg.Environment)

	// Setup database
	db, err := setupDatabase(cfg)
	if err != nil {
		slog.Error("Failed to setup database", "error", err)
		os.Exit(1)
	}

	// Setup dependencies
	deps, err := setupDependencies(db, cfg)
	if err != nil {
		slog.Error("Failed to setup dependencies", "error", err)
		os.Exit(1)
	}

	// Setup HTTP server
	server := setupHTTPServer(deps, cfg)

	// Start server with graceful shutdown
	startServerWithGracefulShutdown(server, cfg.Server.Port)
}

func setupLogger(env string) {
	var level slog.Level
	switch env {
	case "production":
		level = slog.LevelInfo
	case "development":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if env == "development" {
		// Use text handler for development (more readable)
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Use JSON handler for production
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
	slog.Info("Logger initialized", "environment", env, "level", level)
}

func setupDatabase(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.Port,
		cfg.Database.SSLMode,
		cfg.Database.TimeZone,
	)

	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Silent)
	if cfg.Environment == "development" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		// Add performance optimizations
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
		// Add naming strategy if needed
		NamingStrategy: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Auto-migrate tables
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	slog.Info("Database connection established and migrations completed")
	return db, nil
}

func runMigrations(db *gorm.DB) error {
	// List all models to migrate
	models := []interface{}{
		&domain.User{},
		&domain.WaktuKonsultasi{},
		// Add other models here as they are created
	}

	// Run migrations
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate model %T: %w", model, err)
		}
	}

	slog.Info("Database migrations completed successfully")
	return nil
}

type Dependencies struct {
	UserHandler         *userHandler.UserHandler
	AvailabilityHandler *availabilityHandler.AvailabilityHandler
	Config              *config.Config
	Validator           *validator.Validate
	DB                  *gorm.DB
}

func setupDependencies(db *gorm.DB, cfg *config.Config) (*Dependencies, error) {
	// Initialize validator
	validate := validator.New()

	// Setup repositories
	userRepository := userRepo.NewUserRepository(db)
	availabilityRepository := userRepo.NewAvailabilityRepository(db)

	// Setup use cases
	userUsecase := userUsecase.NewUserUsecase(
		userRepository,
		cfg.JWT.Secret,
		cfg.JWT.ExpirationHours,
	)
	availabilityUsecase := usecase.NewAvailabilityUsecase(availabilityRepository) 

	// Setup handlers
	userHandler := userHandler.NewUserHandler(userUsecase, validate)
	availabilityHandler := handler.NewAvailabilityHandler(availabilityUsecase, validate)

	return &Dependencies{
		UserHandler: userHandler,
		AvailabilityHandler: availabilityHandler,
		Config:      cfg,
		Validator:   validate,
		DB:          db,
	}, nil
}

func setupHTTPServer(deps *Dependencies, cfg *config.Config) *http.Server {
	// Set Gin mode based on environment
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Create Gin engine
	engine := gin.New()

	// Add global middleware
	engine.Use(gin.Recovery())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.RateLimiter())

	// Health check endpoint
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0", // You can make this configurable
		})
	})

	// Database health check endpoint
	engine.GET("/health/db", func(c *gin.Context) {
		sqlDB, err := deps.DB.DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "unhealthy",
				"error":  "cannot get database instance",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "unhealthy",
				"error":  "database ping failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"database":  "connected",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Setup API routes
	router.SetupRouter(engine, deps.UserHandler, deps.AvailabilityHandler, cfg.JWT.Secret)

	// Configure HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		// Add additional security headers
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	return server
}

func startServerWithGracefulShutdown(server *http.Server, port string) {
	// Start server in a goroutine
	go func() {
		slog.Info("Server starting",
			"port", port,
			"address", server.Addr,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-quit
	slog.Info("Server shutting down", "signal", sig.String())

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited gracefully")
}
