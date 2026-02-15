package security

import (
	"errors"
	"time"

	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TODO: Move key to .env
var JwtSecretKey = []byte("super-secret-portal-key-change-me")

type PortalClaims struct {
	UserID   string            `json:"uid"`
	ClientID string            `json:"cid"`
	Role     domain.PortalRole `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for the portal user
func GenerateToken(userID, clientID string, role domain.PortalRole) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token valid for 1 day

	claims := &PortalClaims{
		UserID:   userID,
		ClientID: clientID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "az-wap-portal",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JwtSecretKey)
}

// ValidateToken parses and validates a JWT token
func ValidateToken(tokenString string) (*PortalClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PortalClaims{}, func(token *jwt.Token) (interface{}, error) {
		return JwtSecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*PortalClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// HashPassword encrypts the password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash verifies if the password matches the hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
