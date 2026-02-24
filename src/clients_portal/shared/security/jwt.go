package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/AzielCF/az-wap/clients_portal/auth/domain"
	coreconfig "github.com/AzielCF/az-wap/core/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func getSecretKey() []byte {
	return []byte(coreconfig.Global.Security.PortalJWTSecret)
}

type PortalClaims struct {
	UserID   string            `json:"uid"`
	ClientID string            `json:"cid"`
	Role     domain.PortalRole `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT for the portal user
func GenerateToken(userID, clientID string, role domain.PortalRole) (string, error) {
	expirationTime := time.Now().Add(30 * time.Minute) // Short session for security

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
	return token.SignedString(getSecretKey())
}

// GenerateMagicToken creates a short-lived (15m) token for passwordless login
func GenerateMagicToken(userID, clientID string) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute)

	claims := &PortalClaims{
		UserID:   userID,
		ClientID: clientID,
		Role:     domain.RoleMember, // Magic link users have minimal role initially
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getSecretKey())
}

// GenerateOpaqueToken creates a short, random opaque string for magic links
func GenerateOpaqueToken() (string, error) {
	b := make([]byte, 16) // 32 chars in hex, much shorter than JWT
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateToken parses and validates a JWT token
func ValidateToken(tokenString string) (*PortalClaims, error) {
	// 2. Validate token (With 1m leeway for clock skew)
	token, err := jwt.ParseWithClaims(tokenString, &PortalClaims{}, func(token *jwt.Token) (interface{}, error) {
		return getSecretKey(), nil
	}, jwt.WithLeeway(time.Minute))

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
