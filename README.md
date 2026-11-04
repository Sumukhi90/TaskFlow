# TaskFlow

TaskFlow is a local full-stack task manager: React/Vite in the browser, a Go `net/http` JSON API, and SQLite for persistent application data.

## Features

- Login/logout with database-backed sessions and PBKDF2-SHA256 password hashes
- User-scoped tasks and projects, task tags, project progress and task counts
- Dashboard statistics, search, status/priority/project/date filters, sorting, overdue markers
- Month calendar, analytics charts, activity history, profile and light/dark theme setting
- Validation, confirmation for deletes, loading/error/empty states, and retry actions
- Additive SQLite schema migration that preserves existing task rows

## Start locally (Windows)

From this project folder run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\start.ps1
```

Open `http://localhost:5173`. The API listens on `http://localhost:8080`. The launcher starts whichever service is missing, installs frontend packages on first use, checks `/health` and the UI, and writes logs under `.runlogs/`.

Demo login:

- Email: `demo@taskflow.com`
- Password: `demo123`

For manual startup, run `cd backend; go run .` and `cd frontend; npm.cmd install; npm.cmd run dev` in separate terminals. Go and Node.js/npm are required. `TASKFLOW_DB` selects a database path (default `backend/taskflow.db` when launched by the script); `TASKFLOW_ADDR` selects the API listen address; `VITE_API_URL` overrides the frontend API base URL (default `http://localhost:8080/api`).

## Database and migration

SQLite stores users, password hashes, sessions, projects, tasks and activity events in `backend/taskflow.db`. The database is created on first start. Existing task tables gain nullable `user_id`, nullable `project_id` and a `tags` JSON-text column. Existing unowned tasks are assigned to the demo account; task IDs and task content are preserved. A fresh database receives a demo account, three starter projects and four sample tasks. Sample tasks are inserted only when the demo account has no tasks. Schema migration version is recorded in `schema_migrations`.

Sessions use random tokens in HttpOnly, SameSite=Lax cookies; only token hashes are stored in SQLite. API data routes require a session. For local HTTP development the cookie is not marked Secure. The frontend sends credentials and the API allows the local Vite origins.

## API overview

All `/api` endpoints except login/logout/me require the session cookie. JSON errors have the shape `{"error":"message"}`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/auth/login` | Sign in and issue session cookie |
| POST | `/api/auth/logout` | Revoke session and clear cookie |
| GET | `/api/auth/me` | Current signed-in user |
| GET/PUT | `/api/profile` | Read or update name and theme |
| GET/POST | `/api/tasks` | List/create tasks; supports search and status/priority/project/date filters and sorting |
| GET/PUT/DELETE | `/api/tasks/{id}` | Read/update/delete one task |
| GET | `/api/stats` | Dashboard totals and status counts |
| GET/POST | `/api/projects` | List/create projects |
| GET/PUT/DELETE | `/api/projects/{id}` | Read with tasks/update/delete project |
| GET | `/api/calendar?month=YYYY-MM` | Tasks for a calendar month |
| GET | `/api/analytics` | Task and project metrics |
| GET | `/api/activity` | Recent task/project changes |
| GET | `/health` | Public process health check for the launcher |

Task dates use `YYYY-MM-DD`, status is `todo`, `in_progress` or `completed`, and priority is `low`, `medium` or `high`.

## Project structure

```text
backend/
  main.go          # routing, middleware, server startup
  database.go      # schema creation, additive migration and initial demo seed
  auth.go          # password hashing, sessions, profile
  tasks.go         # task validation, filtering, CRUD and persistence
  projects.go      # project CRUD and task association
  insights.go      # dashboard, calendar, analytics and activity queries
  models.go        # API/database models
  main_test.go     # Go API and migration tests
  taskflow.db      # generated local SQLite database
frontend/src/
  App.jsx          # screens, forms and UI state
  services.js      # credentialed API client
  index.css        # responsive layout and theme styles
start.ps1         # starts and checks both local services
README.md
```

## Verify

```powershell
cd backend; go test ./...
cd ..\frontend; npm.cmd run build
```

Tests use isolated in-memory SQLite databases. They cover authentication/protection, CRUD validation, project operations, insights/profile and migration preservation.

## Viva guide

1. `backend/main.go` — HTTP routes, auth middleware, CORS and startup.
2. `backend/database.go` — additive migration and seed-on-empty rules.
3. `backend/auth.go` — salted password hashing and persistent sessions.
4. `backend/tasks.go` — request validation, SQL filters and CRUD.
5. `backend/projects.go` — project ownership and task association.
6. `backend/insights.go` — SQL-backed dashboard/calendar/analytics/activity.
7. `backend/models.go` — request and response structures.
8. `backend/main_test.go` — API and migration tests.
9. `frontend/src/services.js` — browser-to-API requests with credentials.
10. `frontend/src/App.jsx` and `frontend/src/index.css` — React screens, state and responsive UI.
11. `start.ps1` — one-command local launch.

## Scope limits

This is a local demo login, without account registration, password reset, email verification or production HTTPS deployment. Sessions expire after seven days. SQLite is intended for local/single-instance use.
