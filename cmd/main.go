// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"github.com/X3nonxe/gopsy-backend/internal/config"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/handler"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/middleware"
	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/router"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/repository"
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
	zapLogger, err := setupZapLogger(cfg.Environment)
	if err != nil {
		slog.Error("Failed to setup zap logger", "error", err)
		os.Exit(1)
	}
	defer zapLogger.Sync()

	// Setup database
	db, err := setupDatabase(cfg, zapLogger)
	if err != nil {
		zapLogger.Error("Failed to setup database", zap.Error(err))
		os.Exit(1)
	}

	// Setup dependencies
	deps, err := setupDependencies(db, cfg, zapLogger)
	if err != nil {
		zapLogger.Error("Failed to setup dependencies", zap.Error(err))
		os.Exit(1)
	}

	// Setup HTTP server
	server := setupHTTPServer(deps, cfg, zapLogger)

	// Start server with graceful shutdown
	startServerWithGracefulShutdown(server, cfg.Server.Port, zapLogger)
}

func setupZapLogger(env string) (*zap.Logger, error) {
	var config zap.Config

	if env == "production" {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Common configuration
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// Build logger
	logger, err := config.Build(zap.AddCallerSkip(0))
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	logger.Info("Logger initialized",
		zap.String("environment", env),
		zap.String("level", config.Level.String()),
	)

	return logger, nil
}

func setupDatabase(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
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

	// Configure GORM logger based on environment
	var gormLogLevel gormLogger.LogLevel
	switch cfg.Environment {
	case "production":
		gormLogLevel = gormLogger.Error
	case "development":
		gormLogLevel = gormLogger.Info
	default:
		gormLogLevel = gormLogger.Warn
	}
	gormLoggerInstance := gormLogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormLogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormLogLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   gormLoggerInstance,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: false, // Enable FK constraints for data integrity
		SkipDefaultTransaction:                   true,  // Skip default transaction for better performance
		NamingStrategy:                           nil,   // You can add custom naming strategy if needed
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings with proper defaults
	maxIdleConns := cfg.Database.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 10
	}
	maxOpenConns := cfg.Database.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 100
	}
	connMaxLifetime := cfg.Database.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = 3600 // 1 hour
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	// Test the connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := runMigrations(db, logger); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	logger.Info("Database connection established and migrations completed",
		zap.String("host", cfg.Database.Host),
		zap.String("database", cfg.Database.Name),
		zap.Int("max_idle_conns", maxIdleConns),
		zap.Int("max_open_conns", maxOpenConns),
	)

	return db, nil
}

func runMigrations(db *gorm.DB, logger *zap.Logger) error {
	logger.Info("Starting database migrations...")

	// List all models to migrate in proper order (considering foreign keys)
	models := []interface{}{
		&domain.User{},
		&domain.WaktuKonsultasi{},
		// Add other models here as they are created
		// Make sure to maintain proper order for foreign key dependencies
	}

	// Run migrations
	for _, model := range models {
		logger.Debug("Migrating model", zap.String("model", fmt.Sprintf("%T", model)))
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate model %T: %w", model, err)
		}
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// Dependencies holds all application dependencies
type Dependencies struct {
	UserHandler         *handler.UserHandler
	AvailabilityHandler *handler.AvailabilityHandler
	Config              *config.Config
	Validator           *validator.Validate
	DB                  *gorm.DB
	Logger              *zap.Logger
}

func setupDependencies(db *gorm.DB, cfg *config.Config, logger *zap.Logger) (*Dependencies, error) {
	// Initialize validator with custom validation rules
	validate := validator.New()

	// You can register custom validation rules here
	// validate.RegisterValidation("custom_rule", customValidationFunc)

	// Setup repositories with logger
	userRepository := repository.NewUserRepository(db, logger)
	availabilityRepository := repository.NewAvailabilityRepository(db, logger)

	// Setup use cases with logger
	userUsecase := usecase.NewUserUsecase(
		userRepository,
		cfg.JWT.Secret,
		cfg.JWT.ExpirationHours,
		logger,
	)
	availabilityUsecase := usecase.NewAvailabilityUsecase(availabilityRepository, logger)

	// Setup handlers with logger
	userHandler := handler.NewUserHandler(userUsecase, validate, logger)
	availabilityHandler := handler.NewAvailabilityHandler(availabilityUsecase, validate, logger)

	logger.Info("Dependencies initialized successfully")

	return &Dependencies{
		UserHandler:         userHandler,
		AvailabilityHandler: availabilityHandler,
		Config:              cfg,
		Validator:           validate,
		DB:                  db,
		Logger:              logger,
	}, nil
}

func setupHTTPServer(deps *Dependencies, cfg *config.Config, logger *zap.Logger) *http.Server {
	// Set Gin mode based on environment
	switch cfg.Environment {
	case "production":
		gin.SetMode(gin.ReleaseMode)
	case "development":
		gin.SetMode(gin.DebugMode)
	default:
		gin.SetMode(gin.TestMode)
	}

	// Create Gin engine with custom configuration
	engine := gin.New()

	// Add global middleware in proper order
	engine.Use(gin.Recovery())
	engine.Use(middleware.Logger())                  // Use the middleware without parameters
	engine.Use(middleware.CORS())                    // CORS handling
	engine.Use(middleware.RequestID())               // Request ID for tracing
	engine.Use(middleware.RateLimiter())             // Rate limiting
	engine.Use(middleware.Security())                // Security headers
	engine.Use(middleware.Timeout(30 * time.Second)) // Request timeout

	// Health check endpoints
	setupHealthChecks(engine, deps.DB, logger)

	// Setup API routes
	router.SetupRouter(engine, deps.UserHandler, deps.AvailabilityHandler, cfg.JWT.Secret)

	// Configure HTTP server with proper timeouts
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		// Additional security configurations
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	logger.Info("HTTP server configured",
		zap.String("address", server.Addr),
		zap.Duration("read_timeout", server.ReadTimeout),
		zap.Duration("write_timeout", server.WriteTimeout),
		zap.Duration("idle_timeout", server.IdleTimeout),
	)

	return server
}

func setupHealthChecks(engine *gin.Engine, db *gorm.DB, logger *zap.Logger) {
	// Basic health check endpoint
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0", // Make this configurable via build flags
			"service":   "gopsy-backend",
		})
	})

	// Liveness probe - simple endpoint to check if app is running
	engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Readiness probe - checks if app is ready to serve traffic
	engine.GET("/health/ready", func(c *gin.Context) {
		// Check database connectivity
		sqlDB, err := db.DB()
		if err != nil {
			logger.Error("Database instance not available", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database instance not available",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			logger.Error("Database ping failed", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database ping failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"database":  "connected",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Database-specific health check
	engine.GET("/health/db", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			logger.Error("Cannot get database instance", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "unhealthy",
				"error":  "cannot get database instance",
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := sqlDB.PingContext(ctx); err != nil {
			logger.Error("Database ping failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "unhealthy",
				"error":  "database ping failed",
			})
			return
		}

		// Get database stats
		stats := sqlDB.Stats()
		c.JSON(http.StatusOK, gin.H{
			"status":           "healthy",
			"database":         "connected",
			"timestamp":        time.Now().Format(time.RFC3339),
			"open_connections": stats.OpenConnections,
			"idle_connections": stats.Idle,
			"in_use":           stats.InUse,
			"wait_count":       stats.WaitCount,
			"wait_duration":    stats.WaitDuration.String(),
		})
	})
}

func startServerWithGracefulShutdown(server *http.Server, port string, logger *zap.Logger) {
	// Start server in a goroutine
	go func() {
		logger.Info("Server starting",
			zap.String("port", port),
			zap.String("address", server.Addr),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal is received
	sig := <-quit
	logger.Info("Server shutting down", zap.String("signal", sig.String()))

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Disable keep-alives to prevent new connections
	server.SetKeepAlivesEnabled(false)

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Server exited gracefully")
}