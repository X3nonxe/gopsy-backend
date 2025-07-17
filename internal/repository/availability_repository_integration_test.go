//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/repository"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupTestDBForAvailability adalah helper untuk koneksi ke DB dan membersihkannya
func setupTestDBForAvailability(t *testing.T) (*gorm.DB, func()) {
	// Muat .env untuk mendapatkan credential DB
	// Path disesuaikan karena file test berada di dalam subdirektori
	if err := godotenv.Load("../../.env"); err != nil {
		log.Fatalf("Error loading .env file for integration tests: %v", err)
	}

	if os.Getenv("DB_HOST") != "localhost" {
		t.Fatalf("DB_HOST must be 'localhost' for integration tests, but got '%s'", os.Getenv("DB_HOST"))
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_NAME"), os.Getenv("DB_PORT"), os.Getenv("DB_SSL_MODE"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database for integration test: %v", err)
	}

	// Migrasi tabel yang diperlukan
	db.AutoMigrate(&domain.User{}, &domain.WaktuKonsultasi{})

	// Fungsi teardown
	teardown := func() {
		// Hapus semua data dari tabel untuk menjaga kebersihan test
		db.Exec("TRUNCATE TABLE users, waktu_konsultasi RESTART IDENTITY CASCADE")
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}

	// Bersihkan tabel sebelum setiap test run
	db.Exec("TRUNCATE TABLE users, waktu_konsultasi RESTART IDENTITY CASCADE")

	return db, teardown
}

func TestAvailabilityRepository_Integration(t *testing.T) {
	db, teardown := setupTestDBForAvailability(t)
	defer teardown()

	availabilityRepo := repository.NewAvailabilityRepository(db)
	ctx := context.Background()

	// Buat user psikolog dummy untuk foreign key
	psikolog := &domain.User{Username: "dr.budi", Email: "budi@test.com", Password: "pwd", Role: "psikolog"}
	db.Create(psikolog)
	assert.NotZero(t, psikolog.ID)

	t.Run("ReplaceAll - From Empty to Many", func(t *testing.T) {
		// Arrange
		slots := []domain.WaktuKonsultasi{
			{PsikologID: psikolog.ID, Hari: "Senin", WaktuMulai: "09:00:00", WaktuSelesai: "12:00:00"},
			{PsikologID: psikolog.ID, Hari: "Selasa", WaktuMulai: "13:00:00", WaktuSelesai: "17:00:00"},
		}

		// Act
		err := availabilityRepo.ReplaceAll(ctx, psikolog.ID, slots)
		assert.NoError(t, err)

		// Assert
		var results []domain.WaktuKonsultasi
		db.Where("psikolog_id = ?", psikolog.ID).Find(&results)
		assert.Len(t, results, 2)
		assert.Equal(t, "Senin", results[0].Hari)
	})

	t.Run("ReplaceAll - From Many to Fewer", func(t *testing.T) {
		// Arrange: Pastikan ada data sebelumnya
		initialSlots := []domain.WaktuKonsultasi{
			{PsikologID: psikolog.ID, Hari: "Senin", WaktuMulai: "09:00:00", WaktuSelesai: "12:00:00"},
			{PsikologID: psikolog.ID, Hari: "Selasa", WaktuMulai: "13:00:00", WaktuSelesai: "17:00:00"},
			{PsikologID: psikolog.ID, Hari: "Rabu", WaktuMulai: "08:00:00", WaktuSelesai: "10:00:00"},
		}
		db.Create(&initialSlots)

		// Arrange: Siapkan data baru yang lebih sedikit
		newSlots := []domain.WaktuKonsultasi{
			{PsikologID: psikolog.ID, Hari: "Jumat", WaktuMulai: "10:00:00", WaktuSelesai: "15:00:00"},
		}

		// Act
		err := availabilityRepo.ReplaceAll(ctx, psikolog.ID, newSlots)
		assert.NoError(t, err)

		// Assert
		var results []domain.WaktuKonsultasi
		db.Where("psikolog_id = ?", psikolog.ID).Find(&results)
		assert.Len(t, results, 1)
		assert.Equal(t, "Jumat", results[0].Hari)
	})

	t.Run("ReplaceAll - From Many to Empty", func(t *testing.T) {
		// Arrange: Pastikan ada data sebelumnya
		initialSlots := []domain.WaktuKonsultasi{
			{PsikologID: psikolog.ID, Hari: "Kamis", WaktuMulai: "11:00:00", WaktuSelesai: "14:00:00"},
		}
		db.Create(&initialSlots)

		// Act: Panggil dengan slice kosong
		err := availabilityRepo.ReplaceAll(ctx, psikolog.ID, []domain.WaktuKonsultasi{})
		assert.NoError(t, err)

		// Assert
		var count int64
		db.Model(&domain.WaktuKonsultasi{}).Where("psikolog_id = ?", psikolog.ID).Count(&count)
		assert.Equal(t, int64(0), count)
	})
}
