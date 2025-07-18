package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/response"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type AvailabilityHandler struct {
	availabilityUsecase domain.AvailabilityUsecase
	validator           *validator.Validate
	logger              *zap.Logger
}

// NewAvailabilityHandler membuat instance baru dari AvailabilityHandler.
func NewAvailabilityHandler(
	au domain.AvailabilityUsecase,
	v *validator.Validate,
	logger *zap.Logger,
) *AvailabilityHandler {
	return &AvailabilityHandler{
		availabilityUsecase: au,
		validator:           v,
		logger:              logger,
	}
}

// SetAvailability menangani permintaan untuk mengatur jadwal ketersediaan psikolog.
func (h *AvailabilityHandler) SetAvailability(c *gin.Context) {
	// 1. Ambil ID psikolog dari token JWT dengan type assertion yang aman
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Warn("User ID not found in token")
		response.Error(c, http.StatusUnauthorized, "User ID not found in token", nil)
		return
	}

	psikologID, ok := userID.(uint)
	if !ok {
		h.logger.Error("Invalid user ID type in token", zap.Any("userID", userID))
		response.Error(c, http.StatusUnauthorized, "Invalid user ID format", nil)
		return
	}

	// 2. Bind payload JSON dari request body
	var payload domain.SetAvailabilityPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.logger.Warn("Invalid request payload", zap.Error(err))
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// 3. Validasi payload menggunakan validator
	if err := h.validator.Struct(payload); err != nil {
		h.logger.Warn("Validation failed", zap.Error(err))
		response.Error(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	// 4. Panggil usecase untuk memproses logika
	err := h.availabilityUsecase.SetAvailability(c.Request.Context(), psikologID, &payload)
	if err != nil {
		// Error mapping yang lebih spesifik
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			h.logger.Warn("Domain error occurred", zap.Error(err))
			response.Error(c, domainErr.HTTPStatus, domainErr.Message, nil)
			return
		}

		h.logger.Error("Failed to update availability", zap.Error(err), zap.Uint("psikolog_id", psikologID))
		response.Error(c, http.StatusInternalServerError, "Failed to update availability", nil)
		return
	}

	// 5. Kirim response sukses
	h.logger.Info("Availability schedule updated successfully", zap.Uint("psikolog_id", psikologID))
	response.Success(c, http.StatusOK, "Availability schedule updated successfully", nil)
}

// GetAvailability menangani permintaan untuk mendapatkan jadwal ketersediaan psikolog.
func (h *AvailabilityHandler) GetAvailability(c *gin.Context) {
	// Ambil psikolog ID dari path parameter
	psikologIDStr := c.Param("psikolog_id")
	psikologIDInt, err := strconv.ParseUint(psikologIDStr, 10, 32)
	if err != nil {
		h.logger.Warn("Invalid psikolog ID format", zap.String("psikolog_id", psikologIDStr))
		response.Error(c, http.StatusBadRequest, "Invalid psikolog ID format", nil)
		return
	}
	psikologID := uint(psikologIDInt)

	// Panggil usecase untuk mendapatkan jadwal
	availability, err := h.availabilityUsecase.GetAvailability(c.Request.Context(), psikologID)
	if err != nil {
		var domainErr *domain.DomainError
		if errors.As(err, &domainErr) {
			h.logger.Warn("Domain error occurred", zap.Error(err))
			response.Error(c, domainErr.HTTPStatus, domainErr.Message, nil)
			return
		}

		h.logger.Error("Failed to get availability", zap.Error(err), zap.Uint("psikolog_id", psikologID))
		response.Error(c, http.StatusInternalServerError, "Failed to get availability", nil)
		return
	}

	response.Success(c, http.StatusOK, "Availability retrieved successfully", availability)
}
