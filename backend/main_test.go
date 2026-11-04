package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func testServer(t *testing.T) (*Server, http.Handler, *http.Cookie) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000`); err != nil {
		t.Fatal(err)
	}
	if err := initializeDatabase(db); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	mux := http.NewServeMux()
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
	app := cors(mux)
	login := request(app, http.MethodPost, "/api/auth/login", map[string]any{"email": demoEmail, "password": demoPassword}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("demo login: %d %s", login.Code, login.Body)
	}
	cookie := login.Result().Cookies()[0]
	t.Cleanup(func() { db.Close() })
	return s, app, cookie
}

func request(handler http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var encoded []byte
	if body != nil {
		encoded, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestAuthAndProtectedRoutes(t *testing.T) {
	_, handler, cookie := testServer(t)
	unauthorized := request(handler, http.MethodGet, "/api/tasks", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized tasks request: %d", unauthorized.Code)
	}
	me := request(handler, http.MethodGet, "/api/auth/me", nil, cookie)
	if me.Code != http.StatusOK || bytes.Contains(me.Body.Bytes(), []byte("password_hash")) {
		t.Fatalf("current user response: %d %s", me.Code, me.Body)
	}
	bad := request(handler, http.MethodPost, "/api/auth/login", map[string]any{"email": demoEmail, "password": "wrong"}, nil)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login: %d", bad.Code)
	}
	logout := request(handler, http.MethodPost, "/api/auth/logout", nil, cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", logout.Code)
	}
	protected := request(handler, http.MethodGet, "/api/tasks", nil, cookie)
	if protected.Code == http.StatusOK {
		t.Fatal("logged-out session remained valid")
	}
}

func TestTaskCRUDActivityAndValidation(t *testing.T) {
	_, handler, cookie := testServer(t)
	valid := map[string]any{"title": "Study Go", "description": "Read handlers", "due_date": "2099-02-03", "status": "todo", "priority": "high", "tags": []string{"viva", "backend"}}
	bad := request(handler, http.MethodPost, "/api/tasks", map[string]any{"title": "", "due_date": "2025-99-55", "status": "wrong", "priority": "urgent"}, cookie)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid input: %d", bad.Code)
	}
	created := request(handler, http.MethodPost, "/api/tasks", valid, cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var task Task
	if err := json.Unmarshal(created.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.ID == 0 || len(task.Tags) != 2 {
		t.Fatalf("created task fields not returned: %+v", task)
	}
	if got := request(handler, http.MethodGet, "/api/tasks/"+strconv64(task.ID), nil, cookie); got.Code != http.StatusOK {
		t.Fatalf("get: %d", got.Code)
	}
	valid["status"] = "completed"
	if got := request(handler, http.MethodPut, "/api/tasks/"+strconv64(task.ID), valid, cookie); got.Code != http.StatusOK {
		t.Fatalf("update: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodGet, "/api/stats", nil, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte(`"completed":2`)) {
		t.Fatalf("stats: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodGet, "/api/activity", nil, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("completed")) {
		t.Fatalf("activity: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodDelete, "/api/tasks/"+strconv64(task.ID), nil, cookie); got.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", got.Code)
	}
	if got := request(handler, http.MethodGet, "/api/tasks/"+strconv64(task.ID), nil, cookie); got.Code != http.StatusNotFound {
		t.Fatalf("missing task: %d", got.Code)
	}
	if got := request(handler, http.MethodGet, "/api/tasks/nope", nil, cookie); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid task ID: %d", got.Code)
	}
	malformed := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBufferString("{"))
	malformed.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, malformed)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: %d", response.Code)
	}
}

func TestProjectCalendarAnalyticsAndProfile(t *testing.T) {
	_, handler, cookie := testServer(t)
	created := request(handler, http.MethodPost, "/api/projects", map[string]any{"name": "Capstone", "description": "Final year work"}, cookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("project create: %d %s", created.Code, created.Body)
	}
	var project Project
	if err := json.Unmarshal(created.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	taskBody := map[string]any{"title": "Capstone task", "description": "Build demo", "due_date": "2099-07-08", "status": "todo", "priority": "medium", "project_id": project.ID}
	if got := request(handler, http.MethodPost, "/api/tasks", taskBody, cookie); got.Code != http.StatusCreated {
		t.Fatalf("project task create: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodGet, "/api/projects/"+strconv64(project.ID), nil, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("Capstone task")) {
		t.Fatalf("project details: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodGet, "/api/calendar?month=2099-07", nil, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("Capstone task")) {
		t.Fatalf("calendar: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodGet, "/api/analytics", nil, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("Capstone")) {
		t.Fatalf("analytics: %d %s", got.Code, got.Body)
	}
	if got := request(handler, http.MethodPut, "/api/profile", map[string]any{"name": "Alex Student", "theme": "dark"}, cookie); got.Code != http.StatusOK || !bytes.Contains(got.Body.Bytes(), []byte("Alex Student")) {
		t.Fatalf("profile: %d %s", got.Code, got.Body)
	}
}

func TestMigrationPreservesExistingTasks(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE tasks(id INTEGER PRIMARY KEY AUTOINCREMENT,title TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',due_date TEXT NOT NULL,status TEXT NOT NULL,priority TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL); INSERT INTO tasks(title,description,due_date,status,priority,created_at,updated_at) VALUES('Keep my work','Existing row','2099-01-02','todo','medium','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := initializeDatabase(db); err != nil {
		t.Fatal(err)
	}
	var count int
	var title string
	var owner sql.NullInt64
	if err := db.QueryRow(`SELECT COUNT(*),MAX(title),MAX(user_id) FROM tasks`).Scan(&count, &title, &owner); err != nil {
		t.Fatal(err)
	}
	if count != 1 || title != "Keep my work" || !owner.Valid {
		t.Fatalf("legacy task not preserved and owned after migration: count=%d title=%q owner=%v", count, title, owner)
	}
}

func strconv64(value int64) string { return fmt.Sprintf("%d", value) }
