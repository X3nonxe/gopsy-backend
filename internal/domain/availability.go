package domain

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaktuKonsultasi merepresentasikan slot waktu ketersediaan seorang psikolog.
type WaktuKonsultasi struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PsikologID   uint      `json:"psikolog_id" gorm:"not null;index"`
	Hari         string    `json:"hari" gorm:"not null;index"`
	WaktuMulai   string    `json:"waktu_mulai" gorm:"type:time;not null"`
	WaktuSelesai string    `json:"waktu_selesai" gorm:"type:time;not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:PsikologID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// TableName mengembalikan nama tabel untuk model WaktuKonsultasi.
func (WaktuKonsultasi) TableName() string {
	return "waktu_konsultasi"
}

// SlotPayload adalah struktur untuk satu slot waktu dalam request.
type SlotPayload struct {
	Hari         string `json:"hari" validate:"required,oneof=Senin Selasa Rabu Kamis Jumat Sabtu Minggu"`
	WaktuMulai   string `json:"waktu_mulai" validate:"required,datetime=15:04:05"`
	WaktuSelesai string `json:"waktu_selesai" validate:"required,datetime=15:04:05"`
}

// Validate melakukan validasi bisnis pada SlotPayload.
func (s *SlotPayload) Validate() error {
	// Parse waktu untuk validasi
	startTime, err := time.Parse("15:04:05", s.WaktuMulai)
	if err != nil {
		return NewDomainError(http.StatusBadRequest, "Invalid start time format")
	}

	endTime, err := time.Parse("15:04:05", s.WaktuSelesai)
	if err != nil {
		return NewDomainError(http.StatusBadRequest, "Invalid end time format")
	}

	// Validasi waktu mulai harus lebih kecil dari waktu selesai
	if !startTime.Before(endTime) {
		return NewDomainError(http.StatusBadRequest, "Start time must be before end time")
	}

	// Validasi minimal durasi (misalnya 30 menit)
	minDuration := 30 * time.Minute
	if endTime.Sub(startTime) < minDuration {
		return NewDomainError(http.StatusBadRequest, "Minimum consultation duration is 30 minutes")
	}

	return nil
}

// SetAvailabilityPayload adalah payload untuk mengatur jadwal ketersediaan.
type SetAvailabilityPayload struct {
	Slots []SlotPayload `json:"slots" validate:"required,min=1,dive"`
}

// Validate melakukan validasi bisnis pada SetAvailabilityPayload.
func (p *SetAvailabilityPayload) Validate() error {
	// Cek duplikasi hari dan waktu
	daySlots := make(map[string][]SlotPayload)

	for _, slot := range p.Slots {
		// Validasi individual slot
		if err := slot.Validate(); err != nil {
			return err
		}

		// Kelompokkan berdasarkan hari
		daySlots[slot.Hari] = append(daySlots[slot.Hari], slot)
	}

	// Validasi overlapping waktu dalam hari yang sama
	for day, slots := range daySlots {
		if err := validateNoOverlapping(slots); err != nil {
			return NewDomainError(http.StatusBadRequest,
				fmt.Sprintf("Time overlapping detected for %s: %v", day, err))
		}
	}

	return nil
}

// validateNoOverlapping memeriksa apakah ada slot yang overlapping dalam satu hari.
func validateNoOverlapping(slots []SlotPayload) error {
	for i := 0; i < len(slots); i++ {
		for j := i + 1; j < len(slots); j++ {
			if isOverlapping(slots[i], slots[j]) {
				return fmt.Errorf("slots %s-%s and %s-%s are overlapping",
					slots[i].WaktuMulai, slots[i].WaktuSelesai,
					slots[j].WaktuMulai, slots[j].WaktuSelesai)
			}
		}
	}
	return nil
}

// isOverlapping memeriksa apakah dua slot waktu overlapping.
func isOverlapping(slot1, slot2 SlotPayload) bool {
	start1, _ := time.Parse("15:04:05", slot1.WaktuMulai)
	end1, _ := time.Parse("15:04:05", slot1.WaktuSelesai)
	start2, _ := time.Parse("15:04:05", slot2.WaktuMulai)
	end2, _ := time.Parse("15:04:05", slot2.WaktuSelesai)

	return start1.Before(end2) && start2.Before(end1)
}

// DomainError adalah custom error untuk domain logic.
type DomainError struct {
	HTTPStatus int
	Message    string
	Err        error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError membuat instance baru dari DomainError.
func NewDomainError(status int, message string) *DomainError {
	return &DomainError{
		HTTPStatus: status,
		Message:    message,
	}
}

// NewDomainErrorWithCause membuat instance DomainError dengan underlying error.
func NewDomainErrorWithCause(status int, message string, err error) *DomainError {
	return &DomainError{
		HTTPStatus: status,
		Message:    message,
		Err:        err,
	}
}

// AvailabilityRepository mendefinisikan kontrak untuk interaksi database ketersediaan.
type AvailabilityRepository interface {
	ReplaceAll(ctx context.Context, psikologID uint, slots []WaktuKonsultasi) error
	GetByPsikologID(ctx context.Context, psikologID uint) ([]WaktuKonsultasi, error)
	GetByPsikologIDAndDay(ctx context.Context, psikologID uint, day string) ([]WaktuKonsultasi, error)
}

// AvailabilityUsecase mendefinisikan kontrak untuk logika bisnis ketersediaan.
type AvailabilityUsecase interface {
	SetAvailability(ctx context.Context, psikologID uint, payload *SetAvailabilityPayload) error
	GetAvailability(ctx context.Context, psikologID uint) ([]WaktuKonsultasi, error)
	GetAvailabilityByDay(ctx context.Context, psikologID uint, day string) ([]WaktuKonsultasi, error)
}
