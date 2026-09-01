package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		log.Fatal(err)
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {

	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		log.Fatal(err)
	}
	return match, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	timenow := jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  timenow,
		ExpiresAt: jwt.NewNumericDate(timenow.Add(expiresIn)),
		Subject:   userID.String(),
	})
	signedtoken, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return signedtoken, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	type MyCustomClaims struct {
		jwt.RegisteredClaims
	}
	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*MyCustomClaims)
	if !ok {
		return uuid.Nil, errors.New("invalid claims")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("Parsing went wrong.")
	}
	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {

	if headers.Get("Authorization") != "" {
		TOKEN_STRING := strings.TrimPrefix(headers.Get("Authorization"), "Bearer ")
		return TOKEN_STRING, nil
	}
	err := fmt.Errorf("Headers - Authorization is empty")
	return "", err
}

func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	EncodedToString := hex.EncodeToString(key)
	return EncodedToString
}

func GetAPIKEY(headers http.Header) (string, error) {
	if headers.Get("Authorization") != "" {
		ApiKey := strings.TrimPrefix(headers.Get("Authorization"), "ApiKey ")
		return ApiKey, nil
	}
	err := fmt.Errorf("Headers- Authorization is empty")
	return "", err
}
