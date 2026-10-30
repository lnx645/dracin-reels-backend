package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/lnx645/dracin-reels-backend/internal/domain"
)

type AuthService struct {
	store  Store
	secret string
}

func NewAuthService(store Store, secret string) *AuthService {
	return &AuthService{store: store, secret: secret}
}

func (s *AuthService) Register(ctx context.Context, name, email, password string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" || email == "" || len(password) < 6 {
		return nil, domain.ErrBadRequest("VALIDATION_ERROR", "Nama, email, dan kata sandi (minimal 6 karakter) wajib diisi.")
	}
	if _, err := s.store.GetUserByEmail(ctx, email); err == nil {
		return nil, domain.ErrConflict("EMAIL_TAKEN", "Email sudah terdaftar, silakan masuk.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &domain.User{Name: name, Email: email, Role: domain.RoleViewer, PasswordHash: string(hash)}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, *domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return "", nil, domain.ErrUnauthorized("Email atau kata sandi salah.")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, domain.ErrUnauthorized("Email atau kata sandi salah.")
	}
	claims := jwt.MapClaims{
		"sub":  u.ID,
		"role": u.Role,
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	if err != nil {
		return "", nil, err
	}
	u.PasswordHash = ""
	return token, u, nil
}

func (s *AuthService) Me(ctx context.Context, id int64) (*domain.User, error) {
	u, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		return nil, domain.ErrNotFound("Pengguna")
	}
	u.PasswordHash = ""
	return u, nil
}

// ParseToken validates a JWT and returns user id + role.
func (s *AuthService) ParseToken(tokenStr string) (int64, string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("metode token tidak valid")
		}
		return []byte(s.secret), nil
	})
	if err != nil || !token.Valid {
		return 0, "", errors.New("token tidak valid")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", errors.New("token tidak valid")
	}
	sub, _ := claims["sub"].(float64)
	role, _ := claims["role"].(string)
	return int64(sub), role, nil
}
