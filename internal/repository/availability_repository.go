package repository

import (
	"context"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"gorm.io/gorm"
)

type availabilityRepository struct {
	db *gorm.DB
}

// NewAvailabilityRepository membuat instance baru dari availabilityRepository.
func NewAvailabilityRepository(db *gorm.DB) domain.AvailabilityRepository {
	return &availabilityRepository{db: db}
}

// ReplaceAll menghapus semua jadwal lama dan menyisipkan yang baru dalam satu transaksi.
func (r *availabilityRepository) ReplaceAll(ctx context.Context, psikologID uint, slots []domain.WaktuKonsultasi) error {
	// Memulai transaksi
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Hapus semua jadwal lama milik psikolog ini
		if err := tx.Where("psikolog_id = ?", psikologID).Delete(&domain.WaktuKonsultasi{}).Error; err != nil {
			return err // Rollback jika gagal
		}

		// 2. Jika ada slot baru, buat di database
		if len(slots) > 0 {
			if err := tx.Create(&slots).Error; err != nil {
				return err // Rollback jika gagal
			}
		}

		// Commit jika semua berhasil
		return nil
	})
}
