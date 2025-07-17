package domain

import (
	"context"
	"time"
)

// WaktuKonsultasi merepresentasikan slot waktu ketersediaan seorang psikolog.
type WaktuKonsultasi struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	PsikologID uint   `json:"psikolog_id" gorm:"not null;index"`
	Hari       string `json:"hari" gorm:"not null"`
	// === PERBAIKAN UTAMA DI SINI ===
	// Secara eksplisit memberitahu GORM untuk menggunakan tipe data 'time' di database.
	// Ini akan cocok dengan string "HH:MM:SS" yang kita kirim.
	WaktuMulai   string    `json:"waktu_mulai" gorm:"type:time;not null"`
	WaktuSelesai string    `json:"waktu_selesai" gorm:"type:time;not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:PsikologID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// SlotPayload adalah struktur untuk satu slot waktu dalam request.
type SlotPayload struct {
	Hari         string `json:"hari" validate:"required,oneof=Senin Selasa Rabu Kamis Jumat Sabtu Minggu"`
	WaktuMulai   string `json:"waktu_mulai" validate:"required,datetime=15:04:05"`
	WaktuSelesai string `json:"waktu_selesai" validate:"required,datetime=15:04:05"`
}

// SetAvailabilityPayload adalah payload untuk mengatur jadwal ketersediaan.
type SetAvailabilityPayload struct {
	Slots []SlotPayload `json:"slots" validate:"required,min=1,dive"`
}

// AvailabilityRepository mendefinisikan kontrak untuk interaksi database ketersediaan.
type AvailabilityRepository interface {
	ReplaceAll(ctx context.Context, psikologID uint, slots []WaktuKonsultasi) error
}

// AvailabilityUsecase mendefinisikan kontrak untuk logika bisnis ketersediaan.
type AvailabilityUsecase interface {
	SetAvailability(ctx context.Context, psikologID uint, payload *SetAvailabilityPayload) error
}
