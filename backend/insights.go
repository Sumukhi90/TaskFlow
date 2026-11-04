package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

type Metric struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DailyMetric struct {
	Date  string `json:"date"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ProjectMetric struct {
	Name      string `json:"name"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
}

type Analytics struct {
	Stats             Stats           `json:"stats"`
	CompletedThisWeek int             `json:"completed_this_week"`
	Weekly            []DailyMetric   `json:"weekly"`
	ByStatus          []Metric        `json:"by_status"`
	ByPriority        []Metric        `json:"by_priority"`
	ByProject         []ProjectMetric `json:"by_project"`
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	stats, err := s.readStats(requestUserID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load statistics")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) readStats(userID int64) (Stats, error) {
	var stats Stats
	err := s.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status='todo' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='in_progress' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN due_date < date('now','localtime') AND status!='completed' THEN 1 ELSE 0 END),0) FROM tasks WHERE user_id=?`, userID).Scan(&stats.Total, &stats.Todo, &stats.InProgress, &stats.Completed, &stats.Overdue)
	if err == nil && stats.Total > 0 {
		stats.CompletionPercentage = float64(stats.Completed) * 100 / float64(stats.Total)
	}
	return stats, err
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	first, err := time.Parse("2006-01", month)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Month must use YYYY-MM format")
		return
	}
	from := first.Format("2006-01-02")
	to := first.AddDate(0, 1, -1).Format("2006-01-02")
	rows, err := s.db.Query(`SELECT `+taskColumns+` FROM tasks t LEFT JOIN projects p ON p.id=t.project_id WHERE t.user_id=? AND t.due_date BETWEEN ? AND ? ORDER BY t.due_date,t.title`, requestUserID(r), from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load calendar tasks")
		return
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			writeError(w, http.StatusInternalServerError, "Could not load calendar tasks")
			return
		}
		tasks = append(tasks, task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": month, "tasks": tasks})
}

func (s *Server) analytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	userID := requestUserID(r)
	result := Analytics{Weekly: []DailyMetric{}, ByStatus: []Metric{}, ByPriority: []Metric{}, ByProject: []ProjectMetric{}}
	var err error
	result.Stats, err = s.readStats(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	today := time.Now()
	weekStart := today.AddDate(0, 0, -6).Format("2006-01-02")
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE user_id=? AND status='completed' AND date(updated_at)>=?`, userID, weekStart).Scan(&result.CompletedThisWeek); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	weekCounts := map[string]int{}
	rows, err := s.db.Query(`SELECT date(updated_at),COUNT(*) FROM tasks WHERE user_id=? AND status='completed' AND date(updated_at)>=? GROUP BY date(updated_at)`, userID, weekStart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "Could not load analytics")
			return
		}
		weekCounts[date] = count
	}
	rows.Close()
	for daysAgo := 6; daysAgo >= 0; daysAgo-- {
		date := today.AddDate(0, 0, -daysAgo)
		dateKey := date.Format("2006-01-02")
		result.Weekly = append(result.Weekly, DailyMetric{Date: dateKey, Label: date.Format("Mon"), Count: weekCounts[dateKey]})
	}
	if result.ByStatus, err = queryMetrics(s.db, `SELECT status,COUNT(*) FROM tasks WHERE user_id=? GROUP BY status ORDER BY CASE status WHEN 'todo' THEN 0 WHEN 'in_progress' THEN 1 ELSE 2 END`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	if result.ByPriority, err = queryMetrics(s.db, `SELECT priority,COUNT(*) FROM tasks WHERE user_id=? GROUP BY priority ORDER BY CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	projects, err := s.db.Query(`SELECT p.name,COUNT(t.id),COALESCE(SUM(CASE WHEN t.status='completed' THEN 1 ELSE 0 END),0) FROM projects p LEFT JOIN tasks t ON t.project_id=p.id AND t.user_id=p.user_id WHERE p.user_id=? GROUP BY p.id ORDER BY p.name COLLATE NOCASE`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load analytics")
		return
	}
	defer projects.Close()
	for projects.Next() {
		var item ProjectMetric
		if err := projects.Scan(&item.Name, &item.Total, &item.Completed); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load analytics")
			return
		}
		result.ByProject = append(result.ByProject, item)
	}
	writeJSON(w, http.StatusOK, result)
}

func queryMetrics(db *sql.DB, query string, userID int64) ([]Metric, error) {
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	metrics := []Metric{}
	for rows.Next() {
		var metric Metric
		if err := rows.Scan(&metric.Label, &metric.Count); err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 100
	rows, err := s.db.Query(`SELECT id,action,entity_type,entity_id,entity_name,created_at FROM activity_events WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, requestUserID(r), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load activity")
		return
	}
	defer rows.Close()
	items := []ActivityItem{}
	for rows.Next() {
		var item ActivityItem
		var entityID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Action, &item.EntityType, &entityID, &item.EntityName, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load activity")
			return
		}
		if entityID.Valid {
			item.EntityID = &entityID.Int64
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func dayBounds(year int, month time.Month) (string, string) {
	return fmt.Sprintf("%04d-%02d-01", year, month), time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}
