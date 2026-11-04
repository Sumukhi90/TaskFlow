package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

type Server struct{ db *sql.DB }

type contextKey string

const userIDKey contextKey = "taskflow-user-id"

func main() {
	dbPath := os.Getenv("TASKFLOW_DB")
	if dbPath == "" {
		dbPath = "taskflow.db"
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		log.Fatal(err)
	}
	if err := initializeDatabase(db); err != nil {
		log.Fatalf("initialize TaskFlow database: %v", err)
	}

	s := &Server{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/auth/login", s.login)
	mux.HandleFunc("/api/auth/logout", s.logout)
	mux.HandleFunc("/api/auth/me", s.me)
	mux.HandleFunc("/api/profile", s.requireAuth(s.profile))
	mux.HandleFunc("/api/tasks", s.requireAuth(s.tasks))
	mux.HandleFunc("/api/tasks/", s.requireAuth(s.task))
	mux.HandleFunc("/api/stats", s.requireAuth(s.stats))
	mux.HandleFunc("/api/projects", s.requireAuth(s.projects))
	mux.HandleFunc("/api/projects/", s.requireAuth(s.project))
	mux.HandleFunc("/api/calendar", s.requireAuth(s.calendar))
	mux.HandleFunc("/api/analytics", s.requireAuth(s.analytics))
	mux.HandleFunc("/api/activity", s.requireAuth(s.activity))

	addr := os.Getenv("TASKFLOW_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("TaskFlow API listening on %s (database %s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, cors(mux)))
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.userIDFromSession(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Please log in to continue")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

func requestUserID(r *http.Request) int64 { return r.Context().Value(userIDKey).(int64) }

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("request must contain one JSON value")
		}
		return err
	}
	return nil
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, PUT, DELETE")
	writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339) }
