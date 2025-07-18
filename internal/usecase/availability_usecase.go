package usecase

import (
	"context"
	"fmt"
	"net/http"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"go.uber.org/zap"
)

type availabilityUsecase struct {
	availabilityRepo domain.AvailabilityRepository
	logger           *zap.Logger
}

// NewAvailabilityUsecase membuat instance baru dari availabilityUsecase.
func NewAvailabilityUsecase(
	ar domain.AvailabilityRepository,
	logger *zap.Logger,
) domain.AvailabilityUsecase {
	return &availabilityUsecase{
		availabilityRepo: ar,
		logger:           logger,
	}
}

// SetAvailability memproses logika untuk mengatur jadwal ketersediaan.
func (uc *availabilityUsecase) SetAvailability(ctx context.Context, psikologID uint, payload *domain.SetAvailabilityPayload) error {
	// Validasi bisnis payload
	if err := payload.Validate(); err != nil {
		uc.logger.Warn("Payload validation failed", zap.Error(err))
		return err
	}

	// Konversi payload ke entitas domain
	newSlots := make([]domain.WaktuKonsultasi, 0, len(payload.Slots))
	for _, slot := range payload.Slots {
		newSlots = append(newSlots, domain.WaktuKonsultasi{
			PsikologID:   psikologID,
			Hari:         slot.Hari,
			WaktuMulai:   slot.WaktuMulai,
			WaktuSelesai: slot.WaktuSelesai,
		})
	}

	// Panggil repository untuk menggantikan jadwal dalam satu transaksi
	if err := uc.availabilityRepo.ReplaceAll(ctx, psikologID, newSlots); err != nil {
		uc.logger.Error("Failed to replace availability slots",
			zap.Error(err), zap.Uint("psikolog_id", psikologID))
		return domain.NewDomainErrorWithCause(
			http.StatusInternalServerError,
			"Failed to update availability schedule",
			err,
		)
	}

	return nil
}

// GetAvailability mengambil jadwal ketersediaan psikolog.
func (uc *availabilityUsecase) GetAvailability(ctx context.Context, psikologID uint) ([]domain.WaktuKonsultasi, error) {
	slots, err := uc.availabilityRepo.GetByPsikologID(ctx, psikologID)
	if err != nil {
		uc.logger.Error("Failed to get availability",
			zap.Error(err), zap.Uint("psikolog_id", psikologID))
		return nil, domain.NewDomainErrorWithCause(
			http.StatusInternalServerError,
			"Failed to retrieve availability schedule",
			err,
		)
	}

	// Jika tidak ada jadwal, kembalikan empty slice (bukan error)
	if len(slots) == 0 {
		uc.logger.Info("No availability found for psikolog", zap.Uint("psikolog_id", psikologID))
	}

	return slots, nil
}

// GetAvailabilityByDay mengambil jadwal ketersediaan psikolog berdasarkan hari.
func (uc *availabilityUsecase) GetAvailabilityByDay(ctx context.Context, psikologID uint, day string) ([]domain.WaktuKonsultasi, error) {
	// Validasi hari
	validDays := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}
	isValidDay := false
	for _, validDay := range validDays {
		if day == validDay {
			isValidDay = true
			break
		}
	}

	if !isValidDay {
		return nil, domain.NewDomainError(
			http.StatusBadRequest,
			fmt.Sprintf("Invalid day: %s", day),
		)
	}

	slots, err := uc.availabilityRepo.GetByPsikologIDAndDay(ctx, psikologID, day)
	if err != nil {
		uc.logger.Error("Failed to get availability by day",
			zap.Error(err), zap.Uint("psikolog_id", psikologID), zap.String("day", day))
		return nil, domain.NewDomainErrorWithCause(
			http.StatusInternalServerError,
			"Failed to retrieve availability schedule",
			err,
		)
	}

	return slots, nil
}
