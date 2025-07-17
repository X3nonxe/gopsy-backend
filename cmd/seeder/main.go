// cmd/seeder/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/X3nonxe/gopsy-backend/internal/config"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/repository"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Muat konfigurasi dari .env
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Koneksi ke database
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Port, cfg.Database.SSLMode, cfg.Database.TimeZone,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	slog.Info("Database connection established for seeder.")

	// Jalankan seeder untuk admin
	seedAdmin(db)
}

// seedAdmin membuat pengguna admin default jika belum ada.
func seedAdmin(db *gorm.DB) {
	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)

	// Definisikan kredensial admin default
	adminEmail := "admin@example.com"
	adminPassword := "adminpassword"

	// 1. Cek apakah admin sudah ada
	slog.Info("Checking for existing admin user...", "email", adminEmail)
	existingUser, err := userRepo.GetByEmail(ctx, adminEmail)
	if err != nil {
		// Jika ada error selain 'not found', hentikan proses
		if err != domain.ErrUserNotFound {
			slog.Error("Failed to check for existing admin", "error", err)
			return
		}
	}

	if existingUser != nil {
		slog.Info("Admin user already exists. Seeder finished.")
		return
	}

	// 2. Jika admin belum ada, buat yang baru
	slog.Info("Admin user not found. Creating a new one...")

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Failed to hash admin password", "error", err)
		return
	}

	adminUser := &domain.User{
		Username: "admin",
		Email:    adminEmail,
		Password: string(hashedPassword),
		Role:     "admin",
	}

	// 3. Simpan ke database
	if err := userRepo.Create(ctx, adminUser); err != nil {
		slog.Error("Failed to create admin user", "error", err)
		return
	}

	slog.Info("Admin user created successfully!", "email", adminEmail)
}
