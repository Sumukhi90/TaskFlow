package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

type projectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectDetails struct {
	Project Project `json:"project"`
	Tasks   []Task  `json:"tasks"`
}

type activityWriter interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func recordActivity(writer activityWriter, userID int64, action, entityType string, entityID int64, name string) error {
	_, err := writer.Exec(`INSERT INTO activity_events(user_id,action,entity_type,entity_id,entity_name,created_at) VALUES(?,?,?,?,?,?)`, userID, action, entityType, entityID, strings.TrimSpace(name), nowText())
	return err
}

func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	userID := requestUserID(r)
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT p.id,p.name,p.description,COUNT(t.id),COALESCE(SUM(CASE WHEN t.status='completed' THEN 1 ELSE 0 END),0),p.created_at,p.updated_at FROM projects p LEFT JOIN tasks t ON t.project_id=p.id AND t.user_id=p.user_id WHERE p.user_id=? GROUP BY p.id ORDER BY p.name COLLATE NOCASE`, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load projects")
			return
		}
		defer rows.Close()
		projects := []Project{}
		for rows.Next() {
			var project Project
			if err := rows.Scan(&project.ID, &project.Name, &project.Description, &project.TaskCount, &project.CompletedCount, &project.CreatedAt, &project.UpdatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "Could not load projects")
				return
			}
			if project.TaskCount > 0 {
				project.ProgressPercent = project.CompletedCount * 100 / project.TaskCount
			}
			projects = append(projects, project)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load projects")
			return
		}
		writeJSON(w, http.StatusOK, projects)
	case http.MethodPost:
		var input projectInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "Project details are invalid")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		input.Description = strings.TrimSpace(input.Description)
		if message := validateProject(input); message != "" {
			writeError(w, http.StatusBadRequest, message)
			return
		}
		tx, err := s.db.Begin()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not create project")
			return
		}
		created := nowText()
		result, err := tx.Exec(`INSERT INTO projects(user_id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`, userID, input.Name, input.Description, created, created)
		if err != nil {
			tx.Rollback()
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				writeError(w, http.StatusConflict, "A project with that name already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "Could not create project")
			return
		}
		id, err := result.LastInsertId()
		if err != nil || recordActivity(tx, userID, "created", "project", id, input.Name) != nil {
			tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Could not create project")
			return
		}
		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not create project")
			return
		}
		project, err := s.getProject(userID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load created project")
			return
		}
		writeJSON(w, http.StatusCreated, project)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	idText := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Project ID is invalid")
		return
	}
	userID := requestUserID(r)
	switch r.Method {
	case http.MethodGet:
		project, err := s.getProject(userID, id)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Project not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load project")
			return
		}
		rows, err := s.db.Query(`SELECT `+taskColumns+` FROM tasks t LEFT JOIN projects p ON p.id=t.project_id WHERE t.user_id=? AND t.project_id=? ORDER BY t.due_date,t.id`, userID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load project tasks")
			return
		}
		defer rows.Close()
		tasks := []Task{}
		for rows.Next() {
			task, scanErr := scanTask(rows)
			if scanErr != nil {
				writeError(w, http.StatusInternalServerError, "Could not load project tasks")
				return
			}
			tasks = append(tasks, task)
		}
		writeJSON(w, http.StatusOK, projectDetails{Project: project, Tasks: tasks})
	case http.MethodPut:
		var input projectInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "Project details are invalid")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		input.Description = strings.TrimSpace(input.Description)
		if message := validateProject(input); message != "" {
			writeError(w, http.StatusBadRequest, message)
			return
		}
		if _, err := s.db.Exec(`UPDATE projects SET name=?,description=?,updated_at=? WHERE id=? AND user_id=?`, input.Name, input.Description, nowText(), id, userID); err != nil {
			writeError(w, http.StatusConflict, "Could not update project. Check that its name is unique.")
			return
		}
		if err := recordActivity(s.db, userID, "updated", "project", id, input.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "Project was updated but activity could not be recorded")
			return
		}
		project, err := s.getProject(userID, id)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Project not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load updated project")
			return
		}
		writeJSON(w, http.StatusOK, project)
	case http.MethodDelete:
		project, err := s.getProject(userID, id)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Project not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load project")
			return
		}
		if _, err := s.db.Exec(`DELETE FROM projects WHERE id=? AND user_id=?`, id, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not delete project")
			return
		}
		if err := recordActivity(s.db, userID, "deleted", "project", id, project.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "Project was deleted but activity could not be recorded")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getProject(userID, id int64) (Project, error) {
	var project Project
	err := s.db.QueryRow(`SELECT p.id,p.name,p.description,COUNT(t.id),COALESCE(SUM(CASE WHEN t.status='completed' THEN 1 ELSE 0 END),0),p.created_at,p.updated_at FROM projects p LEFT JOIN tasks t ON t.project_id=p.id AND t.user_id=p.user_id WHERE p.user_id=? AND p.id=? GROUP BY p.id`, userID, id).Scan(&project.ID, &project.Name, &project.Description, &project.TaskCount, &project.CompletedCount, &project.CreatedAt, &project.UpdatedAt)
	if err == nil && project.TaskCount > 0 {
		project.ProgressPercent = project.CompletedCount * 100 / project.TaskCount
	}
	return project, err
}

func validateProject(input projectInput) string {
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return "Project name must be between 1 and 80 characters"
	}
	if len([]rune(input.Description)) > 1000 {
		return "Project description must be 1000 characters or fewer"
	}
	return ""
}
