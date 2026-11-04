const base = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'

async function request(path, options = {}) {
  let response
  try {
    response = await fetch(`${base}${path}`, {
      ...options,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...options.headers },
    })
  } catch {
    throw new Error('Cannot connect to TaskFlow. Check that the Go backend is running, then try again.')
  }

  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    const error = new Error(body.error || `TaskFlow request failed (${response.status}).`)
    error.status = response.status
    throw error
  }
  if (response.status === 204) return null
  return response.json()
}

const json = (method, value) => ({ method, body: JSON.stringify(value) })

export const api = {
  login: (credentials) => request('/auth/login', json('POST', credentials)),
  logout: () => request('/auth/logout', { method: 'POST' }),
  me: () => request('/auth/me'),
  tasks: (filters = {}) => {
    const params = new URLSearchParams(Object.entries(filters).filter(([, value]) => value !== '' && value != null))
    return request(`/tasks${params.size ? `?${params}` : ''}`)
  },
  create: (task) => request('/tasks', json('POST', task)),
  update: (id, task) => request(`/tasks/${id}`, json('PUT', task)),
  remove: (id) => request(`/tasks/${id}`, { method: 'DELETE' }),
  stats: () => request('/stats'),
  projects: () => request('/projects'),
  createProject: (project) => request('/projects', json('POST', project)),
  updateProject: (id, project) => request(`/projects/${id}`, json('PUT', project)),
  removeProject: (id) => request(`/projects/${id}`, { method: 'DELETE' }),
  project: (id) => request(`/projects/${id}`),
  calendar: (month) => request(`/calendar?month=${encodeURIComponent(month)}`),
  analytics: () => request('/analytics'),
  activity: () => request('/activity'),
  profile: () => request('/profile'),
  updateProfile: (profile) => request('/profile', json('PUT', profile)),
}
