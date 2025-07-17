package handler

import (
	"net/http"

	"github.com/X3nonxe/gopsy-backend/internal/delivery/http/response"
	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userUsecase domain.UserUsecase
	validator   *validator.Validate
}

func NewUserHandler(uu domain.UserUsecase, v *validator.Validate) *UserHandler {
	return &UserHandler{
		userUsecase: uu,
		validator:   v,
	}
}

func (h *UserHandler) Register(c *gin.Context) {
	var payload domain.RegisterPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Validate payload
	if err := h.validator.Struct(payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	user, err := h.userUsecase.Register(c.Request.Context(), &payload)
	if err != nil {
		statusCode := h.getStatusCodeFromError(err)
		response.Error(c, statusCode, "Registration failed", err)
		return
	}

	// Remove password from response
	userResponse := h.sanitizeUserResponse(user)
	response.Success(c, http.StatusCreated, "User registered successfully", userResponse)
}

func (h *UserHandler) RegisterPsychologist(c *gin.Context) {
	var payload domain.RegisterPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Validate payload
	if err := h.validator.Struct(payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	user, err := h.userUsecase.RegisterPsychologist(c.Request.Context(), &payload)
	if err != nil {
		statusCode := h.getStatusCodeFromError(err)
		response.Error(c, statusCode, "Registration failed", err)
		return
	}

	// Remove password from response
	userResponse := h.sanitizeUserResponse(user)
	response.Success(c, http.StatusCreated, "Psychologist registered successfully", userResponse)
}

func (h *UserHandler) Login(c *gin.Context) {
	var payload domain.LoginPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	// Validate payload
	if err := h.validator.Struct(payload); err != nil {
		response.Error(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	loginResponse, err := h.userUsecase.Login(c.Request.Context(), &payload)
	if err != nil {
		statusCode := h.getStatusCodeFromError(err)
		response.Error(c, statusCode, "Login failed", err)
		return
	}

	response.Success(c, http.StatusOK, "Login successful", loginResponse)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User ID not found", nil)
		return
	}

	// Get full user profile from database
	user, err := h.userUsecase.GetProfile(c.Request.Context(), userID.(uint))
	if err != nil {
		statusCode := h.getStatusCodeFromError(err)
		response.Error(c, statusCode, "Failed to get profile", err)
		return
	}

	userResponse := h.sanitizeUserResponse(user)
	response.Success(c, http.StatusOK, "Profile retrieved successfully", userResponse)
}

// Helper methods
func (h *UserHandler) getStatusCodeFromError(err error) int {
	switch err {
	case domain.ErrEmailAlreadyExists:
		return http.StatusConflict
	case domain.ErrInvalidCredentials:
		return http.StatusUnauthorized
	case domain.ErrUserNotFound:
		return http.StatusNotFound
	case domain.ErrInvalidInput:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (h *UserHandler) sanitizeUserResponse(user *domain.User) *domain.UserResponse {
	return &domain.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
