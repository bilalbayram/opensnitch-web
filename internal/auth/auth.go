package auth

import (
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/evilsocket/opensnitch-web/internal/config"
	"github.com/evilsocket/opensnitch-web/internal/db"
)

// Claims holds the JWT token claims.
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token for the given username.
func GenerateToken(username string, cfg *config.AuthConfig) (string, error) {
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.SessionTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(tokenStr string, cfg *config.AuthConfig) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
		return []byte(cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

// HashPassword returns the bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// CheckPassword compares a plaintext password with a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// EnsureDefaultUser creates the default admin user if no users exist.
func EnsureDefaultUser(database *db.Database, cfg *config.AuthConfig) error {
	count, err := database.WebUserCount()
	if err != nil {
		return err
	}
	if count == 0 {
		if err := database.CreateWebUser(cfg.DefaultUser, cfg.DefaultPassword); err != nil {
			return err
		}
		log.Printf("[auth] Created default user: %s", cfg.DefaultUser)
	}
	return nil
}
