package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
)

// maxTokenRequestBytes caps the POST /auth/token body; credentials are tiny.
const maxTokenRequestBytes = 1 << 10

// AuthHandler serves POST /auth/token. It depends only on the auth
// interfaces, so tests use fakes.
type AuthHandler struct {
	Users   auth.UserStore
	Tokens  auth.TokenIssuer
	Metrics AuthMetrics // optional
}

type tokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int    `json:"expiresIn"` // seconds
}

// Token checks the username and password and returns a signed access token.
// A malformed body is 400; any credential problem is the same 401, so a
// caller can't tell an unknown user from a wrong password.
func (h *AuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	metrics := h.Metrics
	if metrics == nil {
		metrics = noAuthMetrics{}
	}

	var req tokenRequest
	body := http.MaxBytesReader(w, r.Body, maxTokenRequestBytes)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	user, err := h.Users.Authenticate(r.Context(), req.Username, req.Password)
	metrics.AuthAttempt(authMethodPassword, err == nil)
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			LoggerFromContext(r.Context(), nil).Error("authenticate user", "error", err)
		}
		WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}

	tok, err := h.Tokens.Issue(user.Username, user.Scopes)
	if err != nil {
		LoggerFromContext(r.Context(), nil).Error("issue token", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "failed to issue token")
		return
	}

	// Tokens are credentials: never let a proxy or browser cache them.
	w.Header().Set("Cache-Control", "no-store")
	WriteJSON(w, http.StatusOK, tokenResponse{
		AccessToken: tok.Value,
		TokenType:   "Bearer",
		ExpiresIn:   int(tok.ExpiresIn.Seconds()),
	})
}
