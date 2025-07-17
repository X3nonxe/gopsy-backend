package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/X3nonxe/gopsy-backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type userUsecase struct {
	userRepo           domain.UserRepository
	jwtSecret          string
	jwtExpirationHours int
}

func NewUserUsecase(ur domain.UserRepository, jwtSecret string, jwtExpirationHours int) domain.UserUsecase {
	return &userUsecase{
		userRepo:           ur,
		jwtSecret:          jwtSecret,
		jwtExpirationHours: jwtExpirationHours,
	}
}

func (uc *userUsecase) registerUser(ctx context.Context, payload *domain.RegisterPayload, role string) (*domain.User, error) {
	// Normalize email
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))

	// 1. Check if email already exists
	existingUser, err := uc.userRepo.GetByEmail(ctx, payload.Email)

	// If no error, user exists - this is a conflict
	if err == nil && existingUser != nil {
		return nil, domain.ErrEmailAlreadyExists
	}

	// If error is not "user not found", it's a database error
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	// Email is available, proceed with registration

	// 2. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 3. Create new user
	user := &domain.User{
		Username: strings.TrimSpace(payload.Username),
		Email:    payload.Email, // Already normalized above
		Password: string(hashedPassword),
		Role:     role,
	}

	// 4. Save to database
	if err := uc.userRepo.Create(ctx, user); err != nil {
		// Handle database-specific unique constraint violations
		if isDuplicateKeyError(err) {
			return nil, domain.ErrEmailAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

// Helper function to detect duplicate key errors across different databases
func isDuplicateKeyError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "23505") || // PostgreSQL unique violation
		strings.Contains(errStr, "1062") || // MySQL duplicate entry
		strings.Contains(errStr, "unique constraint failed") || // SQLite
		strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "already exists")
}

func (uc *userUsecase) Register(ctx context.Context, payload *domain.RegisterPayload) (*domain.User, error) {
	return uc.registerUser(ctx, payload, "klien")
}

func (uc *userUsecase) RegisterPsychologist(ctx context.Context, payload *domain.RegisterPayload) (*domain.User, error) {
	return uc.registerUser(ctx, payload, "psikolog")
}

func (uc *userUsecase) Login(ctx context.Context, payload *domain.LoginPayload) (*domain.LoginResponse, error) {
	// Normalize email for login
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))

	user, err := uc.userRepo.GetByEmail(ctx, payload.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(payload.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	token, err := uc.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		Token: token,
		User: &domain.UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
	}, nil
}

func (uc *userUsecase) GetProfile(ctx context.Context, userID uint) (*domain.User, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *userUsecase) generateJWT(user *domain.User) (string, error) {
	expirationTime := time.Now().Add(time.Hour * time.Duration(uc.jwtExpirationHours))
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     expirationTime.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.jwtSecret))
}
