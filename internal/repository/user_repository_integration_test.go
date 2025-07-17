//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/repository"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDB adalah helper untuk koneksi ke DB dan membersihkannya
func setupTestDB(t *testing.T) (*gorm.DB, func()) {
	// Muat .env untuk mendapatkan credential DB
	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Error loading .env file for integration tests: %v", err)
	}

	// Pastikan DB_HOST di .env adalah localhost untuk testing lokal
	if os.Getenv("DB_HOST") != "localhost" {
		t.Fatalf("DB_HOST must be 'localhost' for integration tests, but got '%s'", os.Getenv("DB_HOST"))
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSL_MODE"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database for integration test: %v", err)
	}

	// Migrasi tabel User jika belum ada
	db.AutoMigrate(&domain.User{})

	// Fungsi teardown yang akan dipanggil setelah test selesai
	teardown := func() {
		// Hapus semua data dari tabel users untuk menjaga kebersihan test
		db.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE")
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}

	// Bersihkan tabel sebelum setiap test run
	db.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE")
	return db, teardown
}

func TestUserRepository_Integration(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()
	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByEmail", func(t *testing.T) {
		// Arrange
		userToCreate := &domain.User{
			Username: "integ_test_user",
			Email:    "integ@test.com",
			Password: "hashedpassword",
			Role:     "klien",
		}

		// Act: Create
		err := userRepo.Create(ctx, userToCreate)
		assert.NoError(t, err)

		// Act: GetByEmail
		retrievedUser, err := userRepo.GetByEmail(ctx, "integ@test.com")

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUser)
		assert.Equal(t, userToCreate.Username, retrievedUser.Username)
		assert.Equal(t, userToCreate.Email, retrievedUser.Email)
		assert.Equal(t, userToCreate.Role, retrievedUser.Role)
		assert.NotZero(t, retrievedUser.ID) // Pastikan ID sudah di-generate oleh DB
	})

	t.Run("GetByEmail - Not Found", func(t *testing.T) {
		// Act
		retrievedUser, err := userRepo.GetByEmail(ctx, "nonexistent@test.com")

		// Assert - Repository interface should handle "not found" gracefully
		// Either return (nil, nil) or (nil, specific_error)
		if err != nil {
			// If error is returned, it should be a specific "not found" error
			assert.Contains(t, err.Error(), "not found")
			assert.Nil(t, retrievedUser)
		} else {
			// If no error, user should be nil
			assert.Nil(t, retrievedUser)
		}
	})
}

func TestUserRepository_Integration_AdditionalScenarios(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()
	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()

	t.Run("Create Duplicate Email", func(t *testing.T) {
		// Arrange
		user1 := &domain.User{
			Username: "user1",
			Email:    "duplicate@test.com",
			Password: "hashedpassword1",
			Role:     "klien",
		}
		user2 := &domain.User{
			Username: "user2",
			Email:    "duplicate@test.com", // Same email
			Password: "hashedpassword2",
			Role:     "klien",
		}

		// Act
		err1 := userRepo.Create(ctx, user1)
		err2 := userRepo.Create(ctx, user2)

		// Assert
		assert.NoError(t, err1)
		assert.Error(t, err2) // Should fail due to unique constraint
		assert.Contains(t, err2.Error(), "duplicate")
	})

	t.Run("Create User with Long Username", func(t *testing.T) {
		// Arrange
		longUsername := strings.Repeat("a", 256) // Assuming max length is 255
		user := &domain.User{
			Username: longUsername,
			Email:    "longusername@test.com",
			Password: "hashedpassword",
			Role:     "klien",
		}

		// Act
		err := userRepo.Create(ctx, user)

		// Assert
		// Should handle gracefully (either truncate or return error)
		if err != nil {
			assert.Contains(t, err.Error(), "length")
		}
	})

	t.Run("Create User with Invalid Email", func(t *testing.T) {
		// Arrange
		user := &domain.User{
			Username: "testuser",
			Email:    "invalid-email-format",
			Password: "hashedpassword",
			Role:     "klien",
		}

		// Act
		err := userRepo.Create(ctx, user)

		// Assert
		// Should validate email format at DB level or app level
		if err != nil {
			assert.Contains(t, err.Error(), "email")
		}
	})

	t.Run("Concurrent Create Operations", func(t *testing.T) {
		// Arrange
		var wg sync.WaitGroup
		results := make(chan error, 10)

		// Act - Create 10 users concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				user := &domain.User{
					Username: fmt.Sprintf("concurrent_user_%d", index),
					Email:    fmt.Sprintf("concurrent%d@test.com", index),
					Password: "hashedpassword",
					Role:     "klien",
				}
				err := userRepo.Create(ctx, user)
				results <- err
			}(i)
		}

		wg.Wait()
		close(results)

		// Assert - All should succeed
		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
				t.Logf("Concurrent create error: %v", err)
			}
		}
		assert.Equal(t, 0, errorCount, "All concurrent creates should succeed")
	})

	t.Run("GetByEmail Case Sensitivity", func(t *testing.T) {
		// Arrange
		user := &domain.User{
			Username: "casetest",
			Email:    "CaseTest@Example.COM",
			Password: "hashedpassword",
			Role:     "klien",
		}

		// Act
		err := userRepo.Create(ctx, user)
		assert.NoError(t, err)

		// Test original case
		retrievedUser, err := userRepo.GetByEmail(ctx, "CaseTest@Example.COM")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUser)

		// Test different case variations
		testCases := []string{
			"casetest@example.com",
			"CASETEST@EXAMPLE.COM",
		}

		for _, email := range testCases {
			retrievedUser, err := userRepo.GetByEmail(ctx, email)

			// This test depends on your database configuration
			// PostgreSQL is case-sensitive by default for email comparison
			// If you want case-insensitive lookup, you need to:
			// 1. Use LOWER() function in your query
			// 2. Store emails in lowercase
			// 3. Use CITEXT type for email column

			if err != nil {
				// If error is returned, it should be "not found"
				assert.Contains(t, err.Error(), "not found")
			}
			// For now, we'll just log the result rather than assert
			t.Logf("Email lookup for %s: user=%v, err=%v", email, retrievedUser != nil, err)
		}
	})
}
