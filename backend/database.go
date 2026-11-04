package main

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	demoEmail    = "demo@taskflow.com"
	demoPassword = "demo123"
)

func initializeDatabase(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE COLLATE NOCASE, password_hash TEXT NOT NULL, theme TEXT NOT NULL DEFAULT 'light' CHECK(theme IN ('light','dark')), created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS projects (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(user_id,name))`,
		`CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', due_date TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN ('todo','in_progress','completed')), priority TEXT NOT NULL CHECK(priority IN ('low','medium','high')), created_at TEXT NOT NULL, updated_at TEXT NOT NULL, user_id INTEGER REFERENCES users(id), project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL, tags TEXT NOT NULL DEFAULT '[]')`,
		`CREATE TABLE IF NOT EXISTS sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash TEXT NOT NULL UNIQUE, expires_at TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS activity_events (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, action TEXT NOT NULL, entity_type TEXT NOT NULL, entity_id INTEGER, entity_name TEXT NOT NULL, created_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"user_id", "INTEGER REFERENCES users(id)"},
		{"project_id", "INTEGER REFERENCES projects(id) ON DELETE SET NULL"},
		{"tags", "TEXT NOT NULL DEFAULT '[]'"},
	} {
		if err := ensureColumn(db, "tasks", column.name, column.definition); err != nil {
			return err
		}
	}

	var userID int64
	err := db.QueryRow(`SELECT id FROM users WHERE email=?`, demoEmail).Scan(&userID)
	if err == sql.ErrNoRows {
		hash, hashErr := hashPassword(demoPassword)
		if hashErr != nil {
			return hashErr
		}
		result, insertErr := db.Exec(`INSERT INTO users(name,email,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)`, "Demo User", demoEmail, hash, nowText(), nowText())
		if insertErr != nil {
			return insertErr
		}
		userID, err = result.LastInsertId()
	} else if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if _, err = db.Exec(`UPDATE tasks SET user_id=? WHERE user_id IS NULL`, userID); err != nil {
		return err
	}
	if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_user_due ON tasks(user_id,due_date); CREATE INDEX IF NOT EXISTS idx_tasks_user_status ON tasks(user_id,status); CREATE INDEX IF NOT EXISTS idx_events_user_created ON activity_events(user_id,created_at DESC); CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token_hash)`); err != nil {
		return err
	}
	if err := seedDemoProjects(db, userID); err != nil {
		return err
	}
	if err := seedDemoTasksIfEmpty(db, userID); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO schema_migrations(version,applied_at) VALUES(1,?)`, nowText())
	return err
}

func ensureColumn(db *sql.DB, table, name, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primary int
		var columnName, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &dataType, &notNull, &defaultValue, &primary); err != nil {
			rows.Close()
			return err
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if found {
		return nil
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, definition))
	return err
}

func seedDemoProjects(db *sql.DB, userID int64) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM projects WHERE user_id=?`, userID).Scan(&count); err != nil || count != 0 {
		return err
	}
	for _, project := range []struct{ name, description string }{
		{"Semester Project", "Plan and deliver the Go semester project."},
		{"Internship", "Keep internship work and documentation on track."},
		{"Personal", "Everyday goals and personal plans."},
	} {
		if _, err := db.Exec(`INSERT INTO projects(user_id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`, userID, project.name, project.description, nowText(), nowText()); err != nil {
			return err
		}
	}
	return nil
}

func seedDemoTasksIfEmpty(db *sql.DB, userID int64) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE user_id=?`, userID).Scan(&count); err != nil || count != 0 {
		return err
	}
	var semesterID, internshipID, personalID int64
	for _, item := range []struct {
		name string
		into *int64
	}{{"Semester Project", &semesterID}, {"Internship", &internshipID}, {"Personal", &personalID}} {
		if err := db.QueryRow(`SELECT id FROM projects WHERE user_id=? AND name=?`, userID, item.name).Scan(item.into); err != nil {
			return err
		}
	}
	today := time.Now().Format("2006-01-02")
	for _, task := range []struct {
		title, description, status, priority string
		days                                 int
		projectID                            int64
	}{
		{"Prepare Go semester project", "Plan API, data model, and final presentation.", "in_progress", "high", 2, semesterID},
		{"Complete internship documentation", "Collect weekly notes and finish the report.", "todo", "medium", 5, internshipID},
		{"Prepare mid-semester exam", "Review the upcoming units and practice questions.", "todo", "high", 3, personalID},
		{"Review ML project", "Review experiments and update project notes.", "completed", "low", -1, semesterID},
	} {
		due := today
		if task.days >= 0 {
			due = time.Now().AddDate(0, 0, task.days).Format("2006-01-02")
		}
		timestamp := nowText()
		if _, err := db.Exec(`INSERT INTO tasks(user_id,project_id,title,description,due_date,status,priority,tags,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, userID, task.projectID, task.title, task.description, due, task.status, task.priority, `[]`, timestamp, timestamp); err != nil {
			return err
		}
	}
	return nil
}
