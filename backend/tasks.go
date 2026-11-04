package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var taskIDPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

const taskColumns = `t.id,t.title,t.description,t.due_date,t.status,t.priority,t.project_id,COALESCE(p.name,''),t.tags,t.created_at,t.updated_at`

func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTasks(w, r)
	case http.MethodPost:
		s.createTask(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) task(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if !taskIDPattern.MatchString(idText) {
		writeError(w, http.StatusBadRequest, "Task ID must be a positive integer")
		return
	}
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Task ID is invalid")
		return
	}
	switch r.Method {
	case http.MethodGet:
		task, err := s.getTaskForUser(requestUserID(r), id)
		if err != nil {
			writeTaskError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, task)
	case http.MethodPut:
		s.updateTask(w, r, id)
	case http.MethodDelete:
		s.deleteTask(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	userID := requestUserID(r)
	query := r.URL.Query()
	conditions := []string{"t.user_id=?"}
	args := []any{userID}
	if search := strings.TrimSpace(query.Get("search")); search != "" {
		conditions = append(conditions, `(LOWER(t.title) LIKE LOWER(?) OR LOWER(t.description) LIKE LOWER(?) OR LOWER(COALESCE(p.name,'')) LIKE LOWER(?))`)
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	for key, column := range map[string]string{"status": "t.status", "priority": "t.priority"} {
		if value := query.Get(key); value != "" && value != "all" {
			conditions = append(conditions, column+"=?")
			args = append(args, value)
		}
	}
	if value := query.Get("project_id"); value != "" {
		projectID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || projectID < 1 {
			writeError(w, http.StatusBadRequest, "Project ID is invalid")
			return
		}
		conditions = append(conditions, "t.project_id=?")
		args = append(args, projectID)
	}
	for key, operator := range map[string]string{"due_from": ">=", "due_to": "<="} {
		if value := query.Get(key); value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				writeError(w, http.StatusBadRequest, "Due date filter must use YYYY-MM-DD")
				return
			}
			conditions = append(conditions, "t.due_date "+operator+" ?")
			args = append(args, value)
		}
	}
	order := "t.due_date ASC,t.id DESC"
	switch query.Get("sort") {
	case "created":
		order = "t.created_at DESC,t.id DESC"
	case "priority":
		order = "CASE t.priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END,t.due_date ASC"
	case "overdue":
		order = "CASE WHEN t.due_date < date('now','localtime') AND t.status!='completed' THEN 0 ELSE 1 END,t.due_date ASC"
	case "due", "":
	default:
		writeError(w, http.StatusBadRequest, "Sort must be due, created, priority, or overdue")
		return
	}
	statement := `SELECT ` + taskColumns + ` FROM tasks t LEFT JOIN projects p ON p.id=t.project_id WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY ` + order
	rows, err := s.db.Query(statement, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load tasks")
		return
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load tasks")
			return
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load tasks")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

type rowScanner interface{ Scan(...any) error }

func scanTask(row rowScanner) (Task, error) {
	var task Task
	var projectID sql.NullInt64
	var tags string
	err := row.Scan(&task.ID, &task.Title, &task.Description, &task.DueDate, &task.Status, &task.Priority, &projectID, &task.ProjectName, &tags, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return task, err
	}
	if projectID.Valid {
		task.ProjectID = &projectID.Int64
	}
	if err := json.Unmarshal([]byte(tags), &task.Tags); err != nil {
		task.Tags = []string{}
	}
	if task.Tags == nil {
		task.Tags = []string{}
	}
	return task, nil
}

func (s *Server) getTaskForUser(userID, id int64) (Task, error) {
	return scanTask(s.db.QueryRow(`SELECT `+taskColumns+` FROM tasks t LEFT JOIN projects p ON p.id=t.project_id WHERE t.user_id=? AND t.id=?`, userID, id))
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var input TaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid task JSON")
		return
	}
	tags, message := validateTaskInput(&input)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	userID := requestUserID(r)
	if !s.validProject(userID, input.ProjectID) {
		writeError(w, http.StatusBadRequest, "Select a project from your account")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create task")
		return
	}
	result, err := tx.Exec(`INSERT INTO tasks(user_id,project_id,title,description,due_date,status,priority,tags,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, userID, input.ProjectID, strings.TrimSpace(input.Title), strings.TrimSpace(input.Description), input.DueDate, input.Status, input.Priority, tags, nowText(), nowText())
	if err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not create task")
		return
	}
	id, err := result.LastInsertId()
	if err != nil || recordActivity(tx, userID, "created", "task", id, input.Title) != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not create task")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create task")
		return
	}
	task, err := s.getTaskForUser(userID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load created task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request, id int64) {
	var input TaskInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Request body must be valid task JSON")
		return
	}
	tags, message := validateTaskInput(&input)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return
	}
	userID := requestUserID(r)
	if !s.validProject(userID, input.ProjectID) {
		writeError(w, http.StatusBadRequest, "Select a project from your account")
		return
	}
	old, err := s.getTaskForUser(userID, id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load task")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not update task")
		return
	}
	result, err := tx.Exec(`UPDATE tasks SET project_id=?,title=?,description=?,due_date=?,status=?,priority=?,tags=?,updated_at=? WHERE user_id=? AND id=?`, input.ProjectID, strings.TrimSpace(input.Title), strings.TrimSpace(input.Description), input.DueDate, input.Status, input.Priority, tags, nowText(), userID, id)
	if err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not update task")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		tx.Rollback()
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	action := "updated"
	if old.Status != "completed" && input.Status == "completed" {
		action = "completed"
	}
	if err := recordActivity(tx, userID, action, "task", id, input.Title); err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not update task")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not update task")
		return
	}
	task, err := s.getTaskForUser(userID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load updated task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request, id int64) {
	userID := requestUserID(r)
	task, err := s.getTaskForUser(userID, id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load task")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete task")
		return
	}
	if _, err := tx.Exec(`DELETE FROM tasks WHERE user_id=? AND id=?`, userID, id); err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not delete task")
		return
	}
	if err := recordActivity(tx, userID, "deleted", "task", id, task.Title); err != nil {
		tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not delete task")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateTaskInput(input *TaskInput) (string, string) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return "", "Title is required"
	}
	if len([]rune(input.Title)) > 120 {
		return "", "Title must be 120 characters or fewer"
	}
	if len([]rune(input.Description)) > 2000 {
		return "", "Description must be 2000 characters or fewer"
	}
	if input.Status != "todo" && input.Status != "in_progress" && input.Status != "completed" {
		return "", "Status must be todo, in_progress, or completed"
	}
	if input.Priority != "low" && input.Priority != "medium" && input.Priority != "high" {
		return "", "Priority must be low, medium, or high"
	}
	date, err := time.Parse("2006-01-02", input.DueDate)
	if err != nil || date.Format("2006-01-02") != input.DueDate {
		return "", "Due date must be a valid date in YYYY-MM-DD format"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	if len(input.Tags) > 12 {
		return "", "A task can have at most 12 tags"
	}
	seen := map[string]bool{}
	for i, tag := range input.Tags {
		tag = strings.TrimSpace(tag)
		if len([]rune(tag)) > 24 {
			return "", "Tags must be 24 characters or fewer"
		}
		if tag == "" || seen[strings.ToLower(tag)] {
			return "", "Tags must be non-empty and unique"
		}
		input.Tags[i] = tag
		seen[strings.ToLower(tag)] = true
	}
	encoded, err := json.Marshal(input.Tags)
	return string(encoded), ""
}

func (s *Server) validProject(userID int64, projectID *int64) bool {
	if projectID == nil {
		return true
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE user_id=? AND id=?`, userID, *projectID).Scan(&count); err != nil {
		return false
	}
	return count == 1
}

func writeTaskError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "Could not load task")
}
