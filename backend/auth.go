package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookie  = "taskflow_session"
	passwordRounds = 310000
)

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type profileInput struct {
	Name  string `json:"name"`
	Theme string `json:"theme"`
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived, err := pbkdf2.Key(sha256.New, password, salt, passwordRounds, 32)
	if err != nil {
		return "", err
	}
	encode := base64.RawStdEncoding.EncodeToString
	return "pbkdf2-sha256$310000$" + encode(salt) + "$" + encode(derived), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "310000" {
		return false
	}
	decode := base64.RawStdEncoding.DecodeString
	salt, err := decode(parts[2])
	if err != nil || len(salt) < 16 {
		return false
	}
	want, err := decode(parts[3])
	if err != nil || len(want) != 32 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, passwordRounds, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var input loginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Enter a valid email and password")
		return
	}
	var user User
	var passwordHash string
	err := s.db.QueryRow(`SELECT id,name,email,theme,password_hash FROM users WHERE email=?`, strings.TrimSpace(strings.ToLower(input.Email))).Scan(&user.ID, &user.Name, &user.Email, &user.Theme, &passwordHash)
	if err != nil || !verifyPassword(passwordHash, input.Password) {
		writeError(w, http.StatusUnauthorized, "Email or password is incorrect")
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not start your session")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	if _, err := s.db.Exec(`INSERT INTO sessions(user_id,token_hash,expires_at,created_at) VALUES(?,?,?,?)`, user.ID, sessionHash(token), expires.Format(time.RFC3339), nowText()); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not start your session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: false, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int((7 * 24 * time.Hour).Seconds())})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, sessionHash(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	user, err := s.userFromSession(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Please log in to continue")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) userFromSession(r *http.Request) (User, error) {
	var user User
	userID, err := s.userIDFromSession(r)
	if err != nil {
		return user, err
	}
	err = s.db.QueryRow(`SELECT id,name,email,theme FROM users WHERE id=?`, userID).Scan(&user.ID, &user.Name, &user.Email, &user.Theme)
	return user, err
}

func (s *Server) userIDFromSession(r *http.Request) (int64, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return 0, errors.New("missing session")
	}
	var userID int64
	err = s.db.QueryRow(`SELECT user_id FROM sessions WHERE token_hash=? AND expires_at>?`, sessionHash(cookie.Value), nowText()).Scan(&userID)
	return userID, err
}

func sessionHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(hash[:])
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	userID := requestUserID(r)
	switch r.Method {
	case http.MethodGet:
		user, err := s.userFromSession(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Please log in to continue")
			return
		}
		writeJSON(w, http.StatusOK, user)
	case http.MethodPut:
		var input profileInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "Profile details are invalid")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len([]rune(input.Name)) > 80 {
			writeError(w, http.StatusBadRequest, "Name must be between 1 and 80 characters")
			return
		}
		if input.Theme != "light" && input.Theme != "dark" {
			writeError(w, http.StatusBadRequest, "Theme must be light or dark")
			return
		}
		if _, err := s.db.Exec(`UPDATE users SET name=?,theme=?,updated_at=? WHERE id=?`, input.Name, input.Theme, nowText(), userID); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not update profile")
			return
		}
		user, err := s.userFromSession(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load profile")
			return
		}
		writeJSON(w, http.StatusOK, user)
	default:
		methodNotAllowed(w)
	}
}
