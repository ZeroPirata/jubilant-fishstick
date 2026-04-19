package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenProvider interface {
	Generate(userID string) (string, error)
	Validate(token string) (string, error)
}

type JwtManager struct {
	secret     []byte
	expiration time.Duration
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func NewJwtManager(secret string, expiration time.Duration) *JwtManager {
	return &JwtManager{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

var (
	ErrInvalidToken = errors.New("token is invalid")
)

func (m *JwtManager) Generate(userID string) (string, error) {
	expirationTime := time.Now().Add(m.expiration)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "hackton-treino",
			Subject:   userID,
			ID:        uuid.NewString(),
			Audience:  []string{"hackathon-web"}}, // TODO: set audience dinamically
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JwtManager) Validate(tokenStr string) (string, error) {
	tokenParsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", err
	}
	if !tokenParsed.Valid {
		return "", ErrInvalidToken
	}
	if claims, ok := tokenParsed.Claims.(*Claims); ok {
		return claims.UserID, nil
	}
	return "", ErrInvalidToken
}
