package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/X3nonxe/gopsy-backend/internal/mocks"
	"github.com/X3nonxe/gopsy-backend/internal/usecase"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func TestUserUsecase_Register(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(mockCtrl)
	logger := zap.NewNop()
	userUsecase := usecase.NewUserUsecase(mockUserRepo, "test-secret", 3600, logger)

	payload := &domain.RegisterPayload{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	t.Run("Success", func(t *testing.T) {
		// Mock: Email tidak ditemukan (return error not found)
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, domain.ErrUserNotFound).
			Times(1)

		// Mock: Create berhasil
		mockUserRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, user *domain.User) {
				assert.Equal(t, payload.Username, user.Username)
				assert.Equal(t, payload.Email, user.Email)
				assert.Equal(t, "klien", user.Role)
				assert.NotEmpty(t, user.Password)
				assert.NotEqual(t, payload.Password, user.Password)

				// Validasi hash password
				err := bcrypt.CompareHashAndPassword(
					[]byte(user.Password),
					[]byte(payload.Password),
				)
				assert.NoError(t, err)
			}).
			Return(nil).
			Times(1)

		user, err := userUsecase.Register(context.Background(), payload)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, payload.Email, user.Email)
		assert.Equal(t, payload.Username, user.Username)
		assert.Equal(t, "klien", user.Role)
	})

	t.Run("Email Already Exists", func(t *testing.T) {
		existingUser := &domain.User{ID: 1, Email: payload.Email}

		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(existingUser, nil). // User ditemukan
			Times(1)

		user, err := userUsecase.Register(context.Background(), payload)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrEmailAlreadyExists))
		assert.Nil(t, user)
	})

	t.Run("Database Error on GetByEmail", func(t *testing.T) {
		dbError := errors.New("database error")

		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, dbError).
			Times(1)

		user, err := userUsecase.Register(context.Background(), payload)

		assert.Error(t, err)
		assert.Equal(t, dbError, err)
		assert.Nil(t, user)
	})

	t.Run("Database Error on Create", func(t *testing.T) {
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, domain.ErrUserNotFound).
			Times(1)

		dbError := errors.New("create error")
		mockUserRepo.EXPECT().
			Create(gomock.Any(), gomock.Any()).
			Return(dbError).
			Times(1)

		user, err := userUsecase.Register(context.Background(), payload)

		assert.Error(t, err)
		assert.Equal(t, dbError, err)
		assert.Nil(t, user)
	})
}

func TestUserUsecase_Login(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockUserRepo := mocks.NewMockUserRepository(mockCtrl)
	userUsecase := usecase.NewUserUsecase(mockUserRepo, "test-secret", 3600, zap.NewNop())

	password := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &domain.User{
		ID:       1,
		Email:    "test@example.com",
		Password: string(hashedPassword),
		Role:     "klien",
	}

	payload := &domain.LoginPayload{
		Email:    "test@example.com",
		Password: "password123",
	}

	t.Run("Success", func(t *testing.T) {
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(user, nil).
			Times(1)

		response, err := userUsecase.Login(context.Background(), payload)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotEmpty(t, response.Token)
		assert.NotNil(t, response.User)
		assert.Equal(t, user.ID, response.User.ID)
	})

	t.Run("User Not Found", func(t *testing.T) {
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, domain.ErrUserNotFound).
			Times(1)

		response, err := userUsecase.Login(context.Background(), payload)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrInvalidCredentials))
		assert.Nil(t, response)
	})

	t.Run("Incorrect Password", func(t *testing.T) {
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(user, nil).
			Times(1)

		incorrectPayload := &domain.LoginPayload{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		response, err := userUsecase.Login(context.Background(), incorrectPayload)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrInvalidCredentials))
		assert.Nil(t, response)
	})

	t.Run("Database Error", func(t *testing.T) {
		dbError := errors.New("database error")

		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, dbError).
			Times(1)

		response, err := userUsecase.Login(context.Background(), payload)

		assert.Error(t, err)
		assert.Equal(t, dbError, err)
		assert.Nil(t, response)
	})

	t.Run("User Nil Pointer", func(t *testing.T) {
		// Simulasikan repository return nil user tanpa error
		mockUserRepo.EXPECT().
			GetByEmail(gomock.Any(), payload.Email).
			Return(nil, nil). // Tidak error tapi user nil
			Times(1)

		response, err := userUsecase.Login(context.Background(), payload)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrInvalidCredentials))
		assert.Nil(t, response)
	})
}
