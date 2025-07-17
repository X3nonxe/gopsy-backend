package usecase

import (
	"context"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
)

type availabilityUsecase struct {
	availabilityRepo domain.AvailabilityRepository
}

// NewAvailabilityUsecase membuat instance baru dari availabilityUsecase.
func NewAvailabilityUsecase(ar domain.AvailabilityRepository) domain.AvailabilityUsecase {
	return &availabilityUsecase{
		availabilityRepo: ar,
	}
}

// SetAvailability memproses logika untuk mengatur jadwal ketersediaan.
func (uc *availabilityUsecase) SetAvailability(ctx context.Context, psikologID uint, payload *domain.SetAvailabilityPayload) error {
	// Konversi payload ke entitas domain
	var newSlots []domain.WaktuKonsultasi
	for _, slot := range payload.Slots {
		// Di sini Anda bisa menambahkan validasi bisnis tambahan jika perlu,
		// misalnya memastikan waktu_mulai < waktu_selesai.
		newSlots = append(newSlots, domain.WaktuKonsultasi{
			PsikologID:   psikologID,
			Hari:         slot.Hari,
			WaktuMulai:   slot.WaktuMulai,
			WaktuSelesai: slot.WaktuSelesai,
		})
	}

	// Panggil repository untuk menggantikan jadwal dalam satu transaksi
	return uc.availabilityRepo.ReplaceAll(ctx, psikologID, newSlots)
}
