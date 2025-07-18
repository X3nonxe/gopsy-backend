package repository

import (
	"context"
	"fmt"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type availabilityRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAvailabilityRepository membuat instance baru dari availabilityRepository.
func NewAvailabilityRepository(db *gorm.DB, logger *zap.Logger) domain.AvailabilityRepository {
	return &availabilityRepository{
		db:     db,
		logger: logger,
	}
}

// ReplaceAll menghapus semua jadwal lama dan menyisipkan yang baru dalam satu transaksi.
func (r *availabilityRepository) ReplaceAll(ctx context.Context, psikologID uint, slots []domain.WaktuKonsultasi) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Hapus semua jadwal lama milik psikolog ini
		if err := tx.Where("psikolog_id = ?", psikologID).Delete(&domain.WaktuKonsultasi{}).Error; err != nil {
			r.logger.Error("Failed to delete old availability slots",
				zap.Error(err), zap.Uint("psikolog_id", psikologID))
			return fmt.Errorf("failed to delete old availability slots: %w", err)
		}

		// 2. Jika ada slot baru, buat di database
		if len(slots) > 0 {
			if err := tx.Create(&slots).Error; err != nil {
				r.logger.Error("Failed to create new availability slots",
					zap.Error(err), zap.Uint("psikolog_id", psikologID))
				return fmt.Errorf("failed to create new availability slots: %w", err)
			}
		}

		r.logger.Info("Successfully replaced availability slots",
			zap.Uint("psikolog_id", psikologID), zap.Int("slots_count", len(slots)))
		return nil
	})
}

// GetByPsikologID mengambil semua jadwal ketersediaan berdasarkan psikolog ID.
func (r *availabilityRepository) GetByPsikologID(ctx context.Context, psikologID uint) ([]domain.WaktuKonsultasi, error) {
	var slots []domain.WaktuKonsultasi

	err := r.db.WithContext(ctx).
		Where("psikolog_id = ?", psikologID).
		Order("hari ASC, waktu_mulai ASC").
		Find(&slots).Error

	if err != nil {
		r.logger.Error("Failed to get availability by psikolog ID",
			zap.Error(err), zap.Uint("psikolog_id", psikologID))
		return nil, fmt.Errorf("failed to get availability: %w", err)
	}

	return slots, nil
}

// GetByPsikologIDAndDay mengambil jadwal ketersediaan berdasarkan psikolog ID dan hari.
func (r *availabilityRepository) GetByPsikologIDAndDay(ctx context.Context, psikologID uint, day string) ([]domain.WaktuKonsultasi, error) {
	var slots []domain.WaktuKonsultasi

	err := r.db.WithContext(ctx).
		Where("psikolog_id = ? AND hari = ?", psikologID, day).
		Order("waktu_mulai ASC").
		Find(&slots).Error

	if err != nil {
		r.logger.Error("Failed to get availability by psikolog ID and day",
			zap.Error(err), zap.Uint("psikolog_id", psikologID), zap.String("day", day))
		return nil, fmt.Errorf("failed to get availability: %w", err)
	}

	return slots, nil
}
