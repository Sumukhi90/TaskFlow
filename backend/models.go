package main

type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Theme string `json:"theme"`
}

type Project struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	TaskCount       int    `json:"task_count"`
	CompletedCount  int    `json:"completed_count"`
	ProgressPercent int    `json:"progress_percentage"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Task struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DueDate     string   `json:"due_date"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	ProjectID   *int64   `json:"project_id"`
	ProjectName string   `json:"project_name"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type TaskInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DueDate     string   `json:"due_date"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	ProjectID   *int64   `json:"project_id"`
	Tags        []string `json:"tags"`
}

type Stats struct {
	Total                int     `json:"total"`
	Todo                 int     `json:"todo"`
	InProgress           int     `json:"in_progress"`
	Completed            int     `json:"completed"`
	Overdue              int     `json:"overdue"`
	CompletionPercentage float64 `json:"completion_percentage"`
}

type ActivityItem struct {
	ID         int64  `json:"id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   *int64 `json:"entity_id"`
	EntityName string `json:"entity_name"`
	CreatedAt  string `json:"created_at"`
}
