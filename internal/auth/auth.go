package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

type Validator struct {
	validKeyHash string
}

func NewValidator(apiKey string) *Validator {
	if apiKey == "" {
		return nil
	}
	hash := sha256.Sum256([]byte(apiKey))
	return &Validator{
		validKeyHash: hex.EncodeToString(hash[:]),
	}
}

func (v *Validator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v == nil {
			next.ServeHTTP(w, r)
			return
		}

		key := extractKey(r)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		if keyHash != v.validKeyHash {
			http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractKey(r *http.Request) string {
	if key := r.Header.Get("X-Api-Key"); key != "" {
		return key
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}