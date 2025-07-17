package handler

import (
	"net/http"

	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/response"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type AvailabilityHandler struct {
	availabilityUsecase domain.AvailabilityUsecase
	validator           *validator.Validate
}

// NewAvailabilityHandler membuat instance baru dari AvailabilityHandler.
func NewAvailabilityHandler(au domain.AvailabilityUsecase, v *validator.Validate) *AvailabilityHandler {
	return &AvailabilityHandler{
		availabilityUsecase: au,
		validator:           v,
	}
}

// SetAvailability menangani permintaan untuk mengatur jadwal ketersediaan psikolog.
func (h *AvailabilityHandler) SetAvailability(c *gin.Context) {
	// 1. Ambil ID psikolog dari token JWT
	userID, exists := c.Get("userID")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User ID not found in token", nil)
		return
	}
	psikologID := userID.(uint)

	// 2. Bind payload JSON dari request body
	var payload domain.SetAvailabilityPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// 3. Validasi payload menggunakan validator
	if err := h.validator.Struct(payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	// 4. Panggil usecase untuk memproses logika
	err := h.availabilityUsecase.SetAvailability(c.Request.Context(), psikologID, &payload)
	if err != nil {
		// Di sini Anda bisa menambahkan pemetaan error yang lebih spesifik jika ada
		response.Error(c, http.StatusInternalServerError, "Failed to update availability", err)
		return
	}

	// 5. Kirim response sukses
	response.Success(c, http.StatusOK, "Availability schedule updated successfully", nil)
}
