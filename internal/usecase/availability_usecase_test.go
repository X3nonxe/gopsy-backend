package usecase_test

import (
	"context"
	"testing"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/mocks" // Pastikan mock sudah di-generate
	"github.com/X3nonxe/gopsy-backend/internal/usecase"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestAvailabilityUsecase_SetAvailability(t *testing.T) {
	// Setup
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockAvailabilityRepo := mocks.NewMockAvailabilityRepository(mockCtrl)
	availabilityUsecase := usecase.NewAvailabilityUsecase(mockAvailabilityRepo)

	ctx := context.Background()
	psikologID := uint(1)

	payload := &domain.SetAvailabilityPayload{
		Slots: []domain.SlotPayload{
			{
				Hari:         "Senin",
				WaktuMulai:   "09:00:00",
				WaktuSelesai: "17:00:00",
			},
			{
				Hari:         "Rabu",
				WaktuMulai:   "10:00:00",
				WaktuSelesai: "15:00:00",
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		// Arrange (Persiapan)
		// Kita harapkan metode ReplaceAll dipanggil sekali dengan psikologID
		// dan slice dari WaktuKonsultasi yang sudah dikonversi.
		// gomock.Any() digunakan karena kita tidak peduli dengan isi slice-nya secara detail,
		// hanya bahwa itu adalah tipe yang benar.
		mockAvailabilityRepo.EXPECT().
			ReplaceAll(ctx, psikologID, gomock.Any()).
			Return(nil).
			Times(1)

		// Act (Eksekusi)
		err := availabilityUsecase.SetAvailability(ctx, psikologID, payload)

		// Assert (Verifikasi)
		assert.NoError(t, err)
	})

	t.Run("Success with Empty Slots (Deletes all)", func(t *testing.T) {
		// Arrange
		emptyPayload := &domain.SetAvailabilityPayload{
			Slots: []domain.SlotPayload{}, // Payload kosong
		}

		// Harapkan ReplaceAll dipanggil dengan slice kosong
		mockAvailabilityRepo.EXPECT().
			ReplaceAll(ctx, psikologID, gomock.Len(0)).
			Return(nil).
			Times(1)

		// Act
		err := availabilityUsecase.SetAvailability(ctx, psikologID, emptyPayload)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Repository Failure", func(t *testing.T) {
		// Arrange
		expectedError := assert.AnError // Error dummy
		mockAvailabilityRepo.EXPECT().
			ReplaceAll(ctx, psikologID, gomock.Any()).
			Return(expectedError).
			Times(1)

		// Act
		err := availabilityUsecase.SetAvailability(ctx, psikologID, payload)

		// Assert
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
	})
}
