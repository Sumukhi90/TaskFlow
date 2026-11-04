import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Activity, ArrowDownWideNarrow, ArrowLeft, ArrowRight, BarChart3, CalendarDays,
  Check, CheckCheck, ChevronDown, Circle, Clock3, FolderKanban, LayoutDashboard,
  ListTodo, LoaderCircle, LogOut, Moon, Plus, Search, Settings, Sparkles, Sun,
  Tag, UserRound, X,
} from 'lucide-react'
import { api } from './services.js'

const statusLabels = { todo: 'To Do', in_progress: 'In Progress', completed: 'Completed' }
const priorityLabels = { low: 'Low', medium: 'Medium', high: 'High' }
const blankTask = { title: '', description: '', due_date: '', status: 'todo', priority: 'medium', project_id: null, tags: [] }
const blankProject = { name: '', description: '' }
const localISO = (date = new Date()) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const prettyDate = (date) => new Date(`${date}T00:00:00`).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
const navItems = [
  ['dashboard', 'Dashboard', LayoutDashboard], ['tasks', 'Tasks', ListTodo],
  ['projects', 'Projects', FolderKanban], ['calendar', 'Calendar', CalendarDays],
  ['analytics', 'Analytics', BarChart3], ['activity', 'Activity', Activity],
]

function App() {
  const [user, setUser] = useState(null)
  const [booting, setBooting] = useState(true)
  const [page, setPage] = useState('dashboard')
  const [tasks, setTasks] = useState([])
  const [stats, setStats] = useState(null)
  const [projects, setProjects] = useState([])
  const [analytics, setAnalytics] = useState(null)
  const [activity, setActivity] = useState([])
  const [selectedTask, setSelectedTask] = useState(null)
  const [editingTask, setEditingTask] = useState(null)
  const [projectForm, setProjectForm] = useState(null)
  const [selectedProject, setSelectedProject] = useState(null)
  const [toast, setToast] = useState(null)
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')

  const refresh = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const [taskData, statData, projectData, analyticsData, activityData] = await Promise.all([
        api.tasks(), api.stats(), api.projects(), api.analytics(), api.activity(),
      ])
      setTasks(taskData)
      setStats(statData)
      setProjects(projectData)
      setAnalytics(analyticsData)
      setActivity(activityData)
      return true
    } catch (error) {
      setLoadError(error.message)
      return false
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    api.me().then((currentUser) => {
      setUser(currentUser)
      return refresh()
    }).catch((error) => {
      if (error.status !== 401) setLoadError(error.message)
    }).finally(() => setBooting(false))
  }, [refresh])

  const notify = (message, retry) => {
    setToast({ message, retry })
    if (!retry) window.setTimeout(() => setToast(null), 3800)
  }

  const signIn = async (credentials) => {
    try {
      const currentUser = await api.login(credentials)
      setUser(currentUser)
      setPage('dashboard')
      notify(`Welcome back, ${currentUser.name.split(' ')[0]}`)
      await refresh()
    } catch (error) {
      throw error
    }
  }

  const signOut = async () => {
    try {
      await api.logout()
      setUser(null)
      setTasks([])
      setStats(null)
      setPage('dashboard')
    } catch (error) {
      notify(error.message, signOut)
    }
  }

  const saveTask = async (form) => {
    try {
      if (editingTask?.id) await api.update(editingTask.id, form)
      else await api.create(form)
      setEditingTask(null)
      setSelectedTask(null)
      notify(editingTask?.id ? 'Task updated' : 'Task created')
      await refresh()
    } catch (error) {
      notify(error.message, () => saveTask(form))
    }
  }

  const deleteTask = async (task) => {
    if (!window.confirm(`Delete “${task.title}”? This action cannot be undone.`)) return
    try {
      await api.remove(task.id)
      setSelectedTask(null)
      notify('Task deleted')
      await refresh()
    } catch (error) {
      notify(error.message, () => deleteTask(task))
    }
  }

  const updateTask = async (task, changes) => {
    try {
      const updated = await api.update(task.id, { ...task, ...changes })
      setSelectedTask(updated)
      notify(changes.status === 'completed' ? 'Task completed' : 'Task updated')
      await refresh()
    } catch (error) {
      notify(error.message, () => updateTask(task, changes))
    }
  }

  const saveProject = async (project) => {
    try {
      if (project.id) await api.updateProject(project.id, project)
      else await api.createProject(project)
      setProjectForm(null)
      notify(project.id ? 'Project updated' : 'Project created')
      await refresh()
    } catch (error) {
      notify(error.message, () => saveProject(project))
    }
  }

  const changePage = (nextPage) => {
    setPage(nextPage)
    setSelectedProject(null)
  }

  if (booting) return <div className="launch-state"><LoaderCircle className="spin" size={24} /> Loading TaskFlow…</div>
  if (!user) return <LoginPage onLogin={signIn} networkError={loadError} onRetry={() => { setBooting(true); api.me().then(setUser).catch((error) => setLoadError(error.message)).finally(() => setBooting(false)) }} />

  return <div className={`app-shell ${user.theme === 'dark' ? 'theme-dark' : ''}`}>
    <aside className="sidebar">
      <a className="brand" href="#dashboard" onClick={(event) => { event.preventDefault(); changePage('dashboard') }}><span className="brand-mark"><CheckCheck size={18} /></span><span>taskflow</span></a>
      <div className="workspace-label">WORKSPACE</div>
      <nav className="side-nav" aria-label="Main navigation">{navItems.map(([key, label, Icon]) => <button key={key} className={`nav-link ${page === key ? 'active' : ''}`} onClick={() => changePage(key)}><Icon size={17} /><span>{label}</span>{key === 'tasks' && <span className="nav-count">{tasks.length}</span>}</button>)}</nav>
      <div className="sidebar-bottom"><div className="workspace-label">ACCOUNT</div>
        <button className={`nav-link ${page === 'profile' ? 'active' : ''}`} onClick={() => changePage('profile')}><UserRound size={17} /><span>Profile</span></button>
        <button className={`nav-link ${page === 'settings' ? 'active' : ''}`} onClick={() => changePage('settings')}><Settings size={17} /><span>Settings</span></button>
        <div className="user-mini"><div className="avatar">{user.name.split(' ').map((part) => part[0]).join('').slice(0, 2).toUpperCase()}</div><div className="user-mini-copy"><strong>{user.name}</strong><span>{user.email}</span></div><button aria-label="Log out" title="Log out" className="icon-button" onClick={signOut}><LogOut size={16} /></button></div>
      </div>
    </aside>
    <main className="main-area">
      <header className="app-header"><div className="mobile-brand"><span className="brand-mark"><CheckCheck size={16} /></span>taskflow</div><div className="breadcrumb">Workspace <span>/</span> {page === 'project-detail' ? selectedProject?.name : pageLabels[page] || 'Dashboard'}</div><div className="header-tools"><span className="today-label"><span className="live-dot" />{new Date().toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}</span><button className="header-user" onClick={() => changePage('profile')} aria-label="Open profile">{user.name.split(' ').map((part) => part[0]).join('').slice(0, 2).toUpperCase()}</button></div></header>
      <nav className="mobile-nav" aria-label="Mobile navigation">{navItems.map(([key, label, Icon]) => <button key={key} className={page === key ? 'active' : ''} onClick={() => changePage(key)}><Icon size={17} /><span>{label}</span></button>)}</nav>
      <div className="page-content">
        {loadError && <div className="page-error" role="alert"><span>{loadError}</span><button onClick={refresh}>Retry</button></div>}
        {page === 'dashboard' && <Dashboard user={user} tasks={tasks} stats={stats} analytics={analytics} activity={activity} loading={loading} onNavigate={changePage} onCreate={() => setEditingTask({ ...blankTask })} onOpenTask={setSelectedTask} />}
        {page === 'tasks' && <TasksPage tasks={tasks} projects={projects} loading={loading} onCreate={() => setEditingTask({ ...blankTask })} onOpenTask={setSelectedTask} onEdit={(task) => setEditingTask(task)} onDelete={deleteTask} onComplete={(task) => updateTask(task, { status: task.status === 'completed' ? 'todo' : 'completed' })} />}
        {page === 'projects' && <ProjectsPage projects={projects} tasks={tasks} onCreate={() => setProjectForm({ ...blankProject })} onEdit={(project) => setProjectForm(project)} onOpen={(project) => { setSelectedProject(project); setPage('project-detail') }} />}
        {page === 'project-detail' && selectedProject && <ProjectDetail project={selectedProject} tasks={tasks.filter((task) => task.project_id === selectedProject.id)} onBack={() => changePage('projects')} onCreateTask={() => setEditingTask({ ...blankTask, project_id: selectedProject.id })} onOpenTask={setSelectedTask} onEditTask={(task) => setEditingTask(task)} onDeleteTask={deleteTask} onCompleteTask={(task) => updateTask(task, { status: task.status === 'completed' ? 'todo' : 'completed' })} />}
        {page === 'calendar' && <CalendarPage onOpenTask={setSelectedTask} />}
        {page === 'analytics' && <AnalyticsPage analytics={analytics} projects={projects} />}
        {page === 'activity' && <ActivityPage items={activity} />}
        {page === 'profile' && <ProfilePage user={user} onSaved={(updated) => { setUser(updated); notify('Profile updated') }} onError={(error) => notify(error.message)} />}
        {page === 'settings' && <SettingsPage user={user} onSaved={(updated) => { setUser(updated); notify('Theme preference saved') }} onError={(error) => notify(error.message)} onLogout={signOut} />}
      </div>
    </main>
    {editingTask && <TaskForm initial={editingTask} projects={projects} onClose={() => setEditingTask(null)} onSubmit={saveTask} />}
    {projectForm && <ProjectForm initial={projectForm} onClose={() => setProjectForm(null)} onSubmit={saveProject} />}
    {selectedTask && <TaskDetails task={selectedTask} onClose={() => setSelectedTask(null)} onEdit={() => { setEditingTask(selectedTask); setSelectedTask(null) }} onDelete={() => deleteTask(selectedTask)} onComplete={() => updateTask(selectedTask, { status: 'completed' })} />}
    {toast && <div role="status" className="toast"><span className="toast-check"><Check size={14} /></span><span>{toast.message}</span>{toast.retry && <button className="retry-toast" onClick={toast.retry}>Retry</button>}<button aria-label="Dismiss notification" onClick={() => setToast(null)}><X size={15} /></button></div>}
  </div>
}

const pageLabels = { dashboard: 'Dashboard', tasks: 'Tasks', projects: 'Projects', calendar: 'Calendar', analytics: 'Analytics', activity: 'Activity', profile: 'Profile', settings: 'Settings' }

function LoginPage({ onLogin, networkError, onRetry }) {
  const [email, setEmail] = useState('demo@taskflow.com')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    try { await onLogin({ email, password }) } catch (loginError) { setError(loginError.message) } finally { setBusy(false) }
  }
  return <main className="login-screen"><div className="login-card"><a className="brand" href="#login"><span className="brand-mark"><CheckCheck size={18} /></span><span>taskflow</span></a><div className="login-kicker"><Sparkles size={14} /> YOUR DAY, IN ORDER</div><h1>Welcome back.</h1><p className="login-subtitle">Sign in to pick up where you left off.</p>{networkError && <div className="form-error" role="alert">{networkError} <button className="inline-retry" onClick={onRetry}>Retry</button></div>}<form onSubmit={submit}><label className="form-field">Email<input type="email" autoComplete="username" required value={email} onChange={(event) => setEmail(event.target.value)} /></label><label className="form-field">Password<input type="password" autoComplete="current-password" required value={password} onChange={(event) => setPassword(event.target.value)} /></label>{error && <div className="form-error" role="alert">{error}</div>}<button className="primary-btn login-submit" type="submit" disabled={busy}>{busy ? <LoaderCircle className="spin" size={16} /> : null}{busy ? 'Signing in…' : 'Sign in'}</button></form><div className="demo-note"><strong>Demo account</strong><span>demo@taskflow.com · password: demo123</span></div><p className="login-foot">A calm space for getting things done.</p></div></main>
}

function Dashboard({ user, tasks, stats, analytics, activity, loading, onNavigate, onCreate, onOpenTask }) {
  const today = localISO()
  const todayTasks = tasks.filter((task) => task.due_date === today && task.status !== 'completed')
  const upcoming = tasks.filter((task) => task.due_date > today && task.due_date <= localISO(new Date(Date.now() + 7 * 86400000)) && task.status !== 'completed').slice(0, 4)
  const hour = new Date().getHours()
  const greeting = hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening'
  return <>
    <div className="welcome-row"><div><div className="eyebrow"><span className="section-marker" /> YOUR OVERVIEW</div><h1>{greeting}, {user.name.split(' ')[0]}</h1><p>Here’s what’s happening with your tasks today.</p></div><button className="primary-btn" onClick={onCreate}><Plus size={17} /> Quick add task</button></div>
    <StatsCards stats={stats} loading={loading} />
    <div className="dashboard-grid"><section className="panel productivity-panel"><PanelHeading title="This week" subtitle="Completed tasks by day" action="View analytics" onAction={() => onNavigate('analytics')} />{loading ? <Skeleton className="chart-skeleton" /> : <WeeklyChart data={analytics?.weekly || []} />}</section><section className="panel focus-panel"><PanelHeading title="Today's tasks" subtitle={todayTasks.length ? `${todayTasks.length} due today` : 'A clear day ahead'} action="All tasks" onAction={() => onNavigate('tasks')} />{todayTasks.length ? <MiniTaskList tasks={todayTasks.slice(0, 4)} onOpenTask={onOpenTask} /> : <InlineEmpty text="Nothing due today. Keep your momentum going." />}</section></div>
    <div className="dashboard-grid lower-grid"><section className="panel"><PanelHeading title="Coming up" subtitle="Next seven days" action="Open calendar" onAction={() => onNavigate('calendar')} />{upcoming.length ? <MiniTaskList tasks={upcoming} onOpenTask={onOpenTask} /> : <InlineEmpty text="No upcoming tasks in the next week." />}</section><section className="panel"><PanelHeading title="Recent activity" subtitle="Your latest updates" action="View activity" onAction={() => onNavigate('activity')} />{activity.length ? <ActivityList items={activity.slice(0, 4)} /> : <InlineEmpty text="Your task activity will show up here." />}</section></div>
  </>
}

function StatsCards({ stats, loading }) {
  const items = [['Total tasks', stats?.total ?? 0, ListTodo, 'soft-sage'], ['To do', stats?.todo ?? 0, Circle, 'soft-blue'], ['In progress', stats?.in_progress ?? 0, Activity, 'soft-amber'], ['Completed', stats?.completed ?? 0, Check, 'soft-green'], ['Overdue', stats?.overdue ?? 0, Clock3, 'soft-red']]
  return <div className="cap-stats">{items.map(([label, value, Icon, tint]) => <article className="cap-stat" key={label}><div className={`cap-stat-icon ${tint}`}><Icon size={17} /></div><span>{label}</span>{loading ? <Skeleton className="stat-skeleton" /> : <strong>{value}</strong>}{label === 'Completed' && !loading && <small>{Math.round(stats?.completion_percentage || 0)}% completion</small>}</article>)}</div>
}

function TasksPage({ tasks, projects, loading, onCreate, onOpenTask, onEdit, onDelete, onComplete }) {
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('all')
  const [priority, setPriority] = useState('all')
  const [project, setProject] = useState('all')
  const [due, setDue] = useState('all')
  const [sort, setSort] = useState('due')
  const today = localISO()
  const filtered = useMemo(() => tasks.filter((task) => {
    const matchesSearch = `${task.title} ${task.description} ${task.project_name}`.toLowerCase().includes(search.toLowerCase())
    const matchesDue = due === 'all' || (due === 'overdue' ? task.due_date < today && task.status !== 'completed' : due === 'today' ? task.due_date === today : task.due_date >= today)
    return matchesSearch && (status === 'all' || task.status === status) && (priority === 'all' || task.priority === priority) && (project === 'all' || String(task.project_id || '') === project) && matchesDue
  }).sort((a, b) => sort === 'recent' ? b.created_at.localeCompare(a.created_at) : sort === 'priority' ? ['high', 'medium', 'low'].indexOf(a.priority) - ['high', 'medium', 'low'].indexOf(b.priority) : sort === 'overdue' ? Number(b.due_date < today && b.status !== 'completed') - Number(a.due_date < today && a.status !== 'completed') || a.due_date.localeCompare(b.due_date) : a.due_date.localeCompare(b.due_date)), [tasks, search, status, priority, project, due, sort, today])
  const clearFilters = () => { setSearch(''); setStatus('all'); setPriority('all'); setProject('all'); setDue('all') }
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> TASK MANAGEMENT</div><h1>Tasks <span className="title-count">{tasks.length}</span></h1><p>Keep the small things moving and the big things in view.</p></div><button className="primary-btn" onClick={onCreate}><Plus size={17} /> New task</button></div>
    <section className="panel task-panel"><div className="task-toolbar"><label className="searchbox"><Search size={16} /><input aria-label="Search tasks" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search tasks, descriptions, projects…" /></label><label className="filter-select"><span>Status</span><select value={status} onChange={(e) => setStatus(e.target.value)}><option value="all">All</option><option value="todo">To Do</option><option value="in_progress">In Progress</option><option value="completed">Completed</option></select></label><label className="filter-select"><span>Priority</span><select value={priority} onChange={(e) => setPriority(e.target.value)}><option value="all">All</option><option value="high">High</option><option value="medium">Medium</option><option value="low">Low</option></select></label><label className="filter-select"><span>Project</span><select value={project} onChange={(e) => setProject(e.target.value)}><option value="all">All</option>{projects.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label className="filter-select"><span>Due date</span><select value={due} onChange={(e) => setDue(e.target.value)}><option value="all">Any</option><option value="today">Today</option><option value="upcoming">Upcoming</option><option value="overdue">Overdue</option></select></label><label className="filter-select sort-select"><span><ArrowDownWideNarrow size={13} /> Sort</span><select value={sort} onChange={(e) => setSort(e.target.value)}><option value="due">Due date</option><option value="recent">Recently created</option><option value="priority">Priority</option><option value="overdue">Overdue first</option></select></label></div>
      {loading ? <div className="task-skeletons">{[1, 2, 3].map((item) => <Skeleton className="row-skeleton" key={item} />)}</div> : filtered.length ? <div className="task-table">{filtered.map((task) => <TaskRow key={task.id} task={task} onOpen={() => onOpenTask(task)} onEdit={() => onEdit(task)} onDelete={() => onDelete(task)} onComplete={() => onComplete(task)} />)}</div> : <EmptyState title={tasks.length ? 'No tasks match those filters' : 'No tasks yet'} detail={tasks.length ? 'Try another search or clear your filters.' : 'Create your first task to get started.'} action={tasks.length ? clearFilters : onCreate} actionLabel={tasks.length ? 'Clear filters' : 'Create a task'} />}
    </section></>
}

function TaskRow({ task, onOpen, onEdit, onDelete, onComplete }) {
  const overdue = task.due_date < localISO() && task.status !== 'completed'
  return <article className={`task-row ${overdue ? 'overdue-row' : ''} ${task.status === 'completed' ? 'done-row' : ''}`}><button className={`task-check ${task.status === 'completed' ? 'is-checked' : ''}`} aria-label={task.status === 'completed' ? `Reopen ${task.title}` : `Complete ${task.title}`} onClick={onComplete}>{task.status === 'completed' && <Check size={13} />}</button><button className="task-open" onClick={onOpen}><span className="task-name-line"><strong>{task.title}</strong>{overdue && <span className="overdue-badge">Overdue</span>}</span><span className="task-description">{task.description || 'No description'}</span><span className="task-meta"><StatusBadge status={task.status} /><PriorityBadge priority={task.priority} />{task.project_name && <span className="project-chip"><FolderKanban size={12} />{task.project_name}</span>}<span className={`due-date ${overdue ? 'due-overdue' : ''}`}><Clock3 size={12} />{prettyDate(task.due_date)}</span>{task.tags?.slice(0, 2).map((tag) => <span className="tag-chip" key={tag}><Tag size={10} />{tag}</span>)}</span></button><div className="task-actions"><button onClick={onEdit} aria-label={`Edit ${task.title}`}>Edit</button><button className="delete-action" onClick={onDelete} aria-label={`Delete ${task.title}`}><X size={16} /></button></div></article>
}

function ProjectsPage({ projects, tasks, onCreate, onEdit, onOpen }) {
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> ORGANIZE YOUR WORK</div><h1>Projects</h1><p>Group related tasks and see your progress at a glance.</p></div><button className="primary-btn" onClick={onCreate}><Plus size={17} /> New project</button></div>{projects.length ? <div className="project-grid">{projects.map((project) => <ProjectCard key={project.id} project={project} tasks={tasks.filter((task) => task.project_id === project.id)} onOpen={() => onOpen(project)} onEdit={() => onEdit(project)} />)}</div> : <EmptyState title="No projects yet" detail="Create a project to group related tasks." action={onCreate} actionLabel="Create a project" />}</>
}

function ProjectCard({ project, tasks, onOpen, onEdit }) {
  const complete = tasks.filter((task) => task.status === 'completed').length
  const percent = tasks.length ? Math.round(complete * 100 / tasks.length) : 0
  return <article className="project-card"><button className="project-card-main" onClick={onOpen}><span className="project-symbol"><FolderKanban size={19} /></span><h2>{project.name}</h2><p>{project.description || 'A place for related tasks.'}</p><div className="project-progress"><div><span>{complete} of {tasks.length} tasks completed</span><strong>{percent}%</strong></div><span className="progress-track"><i style={{ width: `${percent}%` }} /></span></div><span className="project-link">Open project <ArrowRight size={14} /></span></button><button className="project-edit" onClick={onEdit} aria-label={`Edit ${project.name}`}>Edit</button></article>
}

function ProjectDetail({ project, tasks, onBack, onCreateTask, onOpenTask, onEditTask, onDeleteTask, onCompleteTask }) {
  return <><button className="back-link" onClick={onBack}><ArrowLeft size={15} /> All projects</button><div className="page-title-row project-detail-head"><div><div className="eyebrow"><span className="section-marker" /> PROJECT</div><h1>{project.name}</h1><p>{project.description}</p></div><button className="primary-btn" onClick={onCreateTask}><Plus size={17} /> Add task</button></div><section className="panel"><PanelHeading title="Project tasks" subtitle={`${tasks.length} tasks`} />{tasks.length ? <div className="task-table">{tasks.map((task) => <TaskRow key={task.id} task={task} onOpen={() => onOpenTask(task)} onEdit={() => onEditTask(task)} onDelete={() => onDeleteTask(task)} onComplete={() => onCompleteTask(task)} />)}</div> : <InlineEmpty text="No tasks in this project yet." />}</section></>
}

function CalendarPage({ onOpenTask }) {
  const [month, setMonth] = useState(() => { const now = new Date(); return new Date(now.getFullYear(), now.getMonth(), 1) })
  const [monthTasks, setMonthTasks] = useState([])
  const [calendarError, setCalendarError] = useState('')
  const [calendarLoading, setCalendarLoading] = useState(true)
  const [reloadCalendar, setReloadCalendar] = useState(0)
  const year = month.getFullYear()
  const monthIndex = month.getMonth()
  const start = new Date(year, monthIndex, 1)
  const days = new Date(year, monthIndex + 1, 0).getDate()
  const offset = (start.getDay() + 6) % 7
  const monthKey = `${year}-${String(monthIndex + 1).padStart(2, '0')}`
  useEffect(() => { let active = true; setCalendarLoading(true); api.calendar(monthKey).then((data) => { if (active) { setMonthTasks(data.tasks); setCalendarError('') } }).catch((error) => { if (active) setCalendarError(error.message) }).finally(() => { if (active) setCalendarLoading(false) }); return () => { active = false } }, [monthKey, reloadCalendar])
  const grid = Array.from({ length: Math.ceil((offset + days) / 7) * 7 }, (_, index) => { const day = index - offset + 1; const date = day > 0 && day <= days ? `${monthKey}-${String(day).padStart(2, '0')}` : ''; return { day, date, tasks: monthTasks.filter((task) => task.due_date === date) } })
  const moveMonth = (amount) => setMonth(new Date(year, monthIndex + amount, 1))
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> DUE DATE PLANNER</div><h1>Calendar</h1><p>See what’s coming up and open a task for the details.</p></div><div className="calendar-controls"><button className="secondary-btn" onClick={() => moveMonth(-1)} aria-label="Previous month"><ArrowLeft size={15} /></button><strong>{month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</strong><button className="secondary-btn" onClick={() => moveMonth(1)} aria-label="Next month"><ArrowRight size={15} /></button></div></div><section className="panel calendar-panel"><div className="calendar-grid calendar-weekdays">{['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map((day) => <div key={day}>{day}</div>)}</div><div className="calendar-grid">{grid.map((cell, index) => <div className={`calendar-day ${cell.date ? '' : 'outside-day'}`} key={`${cell.date}-${index}`}><span className={cell.date === localISO() ? 'today-number' : ''}>{cell.day > 0 && cell.day <= days ? cell.day : ''}</span><div className="calendar-day-tasks">{cell.tasks.slice(0, 3).map((task) => <button key={task.id} className={`calendar-task status-${task.status}`} onClick={() => onOpenTask(task)} title={task.title}>{task.title}</button>)}{cell.tasks.length > 3 && <small>+{cell.tasks.length - 3} more</small>}</div></div>)}</div>{calendarLoading && <p className="calendar-empty">Loading calendar…</p>}{calendarError && <p className="calendar-empty" role="alert">{calendarError} <button className="panel-action" onClick={() => setReloadCalendar((value) => value + 1)}>Retry</button></p>}{!calendarLoading && !calendarError && !monthTasks.length && <p className="calendar-empty">No tasks scheduled this month. Due dates will appear here.</p>}</section></>
}

function AnalyticsPage({ analytics, projects }) {
  if (!analytics) return <EmptyState title="Analytics are loading" detail="Task insights will appear as soon as the API responds." />
  const maxWeek = Math.max(1, ...analytics.weekly.map((item) => item.count))
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> REAL-TIME INSIGHTS</div><h1>Analytics</h1><p>Progress calculated from your saved tasks.</p></div></div><StatsCards stats={analytics.stats} loading={false} /><div className="analytics-grid"><section className="panel chart-panel"><PanelHeading title="Weekly completions" subtitle={`${analytics.completed_this_week} completed in the last 7 days`} /><div className="bar-chart">{analytics.weekly.map((day) => <div className="bar-column" key={day.date}><span>{day.count}</span><i style={{ height: `${Math.max(5, day.count / maxWeek * 100)}%` }} /><small>{day.label}</small></div>)}</div></section><section className="panel chart-panel"><PanelHeading title="Tasks by status" subtitle="Current workload" /><MetricBars items={analytics.by_status.map((item) => ({ ...item, label: statusLabels[item.label] || item.label, color: item.label === 'completed' ? 'green' : item.label === 'in_progress' ? 'amber' : 'blue' }))} total={analytics.stats.total} /></section><section className="panel chart-panel"><PanelHeading title="Tasks by priority" subtitle="How work is distributed" /><MetricBars items={analytics.by_priority.map((item) => ({ ...item, label: `${priorityLabels[item.label]} priority`, color: item.label }))} total={analytics.stats.total} /></section><section className="panel chart-panel"><PanelHeading title="Progress by project" subtitle="Completed tasks in each project" />{projects.length ? projects.map((project) => <div className="project-metric" key={project.id}><div><span>{project.name}</span><strong>{project.completed_count}/{project.task_count}</strong></div><span className="progress-track"><i style={{ width: `${project.progress_percentage}%` }} /></span></div>) : <InlineEmpty text="Create a project to see progress here." />}</section></div></>
}

function MetricBars({ items, total }) {
  return items.length ? <div className="metric-bars">{items.map((item) => <div className="metric-row" key={item.label}><div><span>{item.label}</span><strong>{item.count}</strong></div><span className="metric-track"><i className={`metric-${item.color}`} style={{ width: `${total ? item.count / total * 100 : 0}%` }} /></span></div>)}</div> : <InlineEmpty text="Saved tasks will appear in this breakdown." />
}

function ActivityPage({ items }) {
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> YOUR WORK HISTORY</div><h1>Activity</h1><p>A timeline of task and project changes.</p></div></div><section className="panel activity-panel">{items.length ? <ActivityList items={items} /> : <EmptyState title="No activity yet" detail="Create or update a task or project to start your timeline." />}</section></>
}

function ActivityList({ items }) {
  const iconFor = (action) => action === 'completed' ? Check : action === 'created' ? Plus : action === 'deleted' ? X : Activity
  const labelFor = (action) => ({ created: 'Created', updated: 'Updated', completed: 'Completed', deleted: 'Deleted' }[action] || action)
  return <div className="activity-list">{items.map((item) => { const Icon = iconFor(item.action); return <div className="activity-item" key={item.id}><span className={`activity-icon activity-${item.action}`}><Icon size={14} /></span><div><p><strong>{labelFor(item.action)}</strong> {item.entity_type} <b>“{item.entity_name}”</b></p><time>{relativeTime(item.created_at)}</time></div></div> })}</div>
}

function relativeTime(value) {
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000))
  if (seconds < 60) return 'Just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} hr ago`
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function ProfilePage({ user, onSaved, onError }) {
  const [name, setName] = useState(user.name)
  const [busy, setBusy] = useState(false)
  const save = async (event) => { event.preventDefault(); setBusy(true); try { const updated = await api.updateProfile({ name, theme: user.theme }); onSaved(updated) } catch (error) { onError(error) } finally { setBusy(false) } }
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> YOUR ACCOUNT</div><h1>Profile</h1><p>Manage the details attached to your workspace.</p></div></div><section className="panel profile-panel"><div className="profile-avatar">{user.name.split(' ').map((part) => part[0]).join('').slice(0, 2).toUpperCase()}</div><form onSubmit={save}><label className="form-field">Name<input value={name} maxLength="80" onChange={(event) => setName(event.target.value)} required /></label><label className="form-field">Email<input value={user.email} readOnly /></label><button className="primary-btn" type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save profile'}</button></form></section></>
}

function SettingsPage({ user, onSaved, onError, onLogout }) {
  const [busy, setBusy] = useState(false)
  const toggleTheme = async () => { setBusy(true); try { onSaved(await api.updateProfile({ name: user.name, theme: user.theme === 'dark' ? 'light' : 'dark' })) } catch (error) { onError(error) } finally { setBusy(false) } }
  return <><div className="page-title-row"><div><div className="eyebrow"><span className="section-marker" /> PREFERENCES</div><h1>Settings</h1><p>Choose how TaskFlow looks and manage your session.</p></div></div><section className="panel settings-list"><div className="setting-row"><div className="setting-icon">{user.theme === 'dark' ? <Moon size={17} /> : <Sun size={17} />}<div><strong>Appearance</strong><span>Choose a light or dark workspace.</span></div></div><button className="secondary-btn" onClick={toggleTheme} disabled={busy}>{user.theme === 'dark' ? 'Switch to light' : 'Switch to dark'}</button></div><div className="setting-row"><div className="setting-icon"><UserRound size={17} /><div><strong>Signed in as {user.name}</strong><span>{user.email}</span></div></div><button className="secondary-btn logout-btn" onClick={onLogout}><LogOut size={15} /> Log out</button></div></section></>
}

function TaskDetails({ task, onClose, onEdit, onDelete, onComplete }) {
  return <ModalFrame title="Task details" onClose={onClose}><div className="detail-title"><h2>{task.title}</h2><StatusBadge status={task.status} /></div><p className="detail-description">{task.description || 'No description was added.'}</p><div className="detail-grid"><Detail label="Due date" value={prettyDate(task.due_date)} /><Detail label="Priority" value={priorityLabels[task.priority]} /><Detail label="Project" value={task.project_name || 'No project'} /><Detail label="Tags" value={task.tags?.join(', ') || 'None'} /><Detail label="Created" value={new Date(task.created_at).toLocaleString()} /><Detail label="Updated" value={new Date(task.updated_at).toLocaleString()} /></div><div className="modal-actions"><button className="secondary-btn danger-btn" onClick={onDelete}>Delete</button><button className="secondary-btn" onClick={onEdit}>Edit</button>{task.status !== 'completed' && <button className="primary-btn" onClick={onComplete}><Check size={15} /> Mark complete</button>}</div></ModalFrame>
}

function TaskForm({ initial, projects, onClose, onSubmit }) {
  const [form, setForm] = useState({ ...blankTask, ...initial })
  const [tagText, setTagText] = useState((initial.tags || []).join(', '))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const editing = Boolean(initial.id)
  const change = (key, value) => setForm((current) => ({ ...current, [key]: value }))
  const submit = async (event) => {
    event.preventDefault()
    if (!form.title.trim()) return setError('Please give your task a title.')
    if (form.title.trim().length > 120) return setError('Keep the title under 120 characters.')
    if (form.description.length > 2000) return setError('Keep the description under 2,000 characters.')
    const date = new Date(`${form.due_date}T00:00:00Z`)
    if (!/^\d{4}-\d{2}-\d{2}$/.test(form.due_date) || Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== form.due_date) return setError('Choose a valid due date.')
    setError('')
    setBusy(true)
    try { await onSubmit({ ...form, title: form.title.trim(), description: form.description.trim(), project_id: form.project_id ? Number(form.project_id) : null, tags: tagText.split(',').map((tag) => tag.trim()).filter(Boolean) }) } catch (submitError) { setError(submitError.message) } finally { setBusy(false) }
  }
  useEscape(onClose)
  return <ModalFrame title={editing ? 'Edit task' : 'Create a task'} eyebrow={editing ? 'KEEP IT MOVING' : 'A FRESH START'} onClose={onClose}><form className="modal-form" onSubmit={submit}><label className="form-field">Title <span className="required">*</span><input autoFocus maxLength="120" value={form.title} onChange={(event) => change('title', event.target.value)} placeholder="What needs to get done?" /></label><label className="form-field">Description <span className="optional">Optional</span><textarea maxLength="2000" rows="3" value={form.description} onChange={(event) => change('description', event.target.value)} placeholder="Add a few details…" /></label><div className="form-grid"><label className="form-field">Due date <span className="required">*</span><input type="date" value={form.due_date} onChange={(event) => change('due_date', event.target.value)} /></label><label className="form-field">Status<select value={form.status} onChange={(event) => change('status', event.target.value)}><option value="todo">To Do</option><option value="in_progress">In Progress</option><option value="completed">Completed</option></select></label></div><div className="form-grid"><label className="form-field">Priority<select value={form.priority} onChange={(event) => change('priority', event.target.value)}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option></select></label><label className="form-field">Project<select value={form.project_id || ''} onChange={(event) => change('project_id', event.target.value || null)}><option value="">No project</option>{projects.map((project) => <option value={project.id} key={project.id}>{project.name}</option>)}</select></label></div><label className="form-field">Tags <span className="optional">Comma separated</span><input value={tagText} onChange={(event) => setTagText(event.target.value)} placeholder="study, exam" /></label>{error && <div className="form-error" role="alert">{error}</div>}<div className="modal-actions"><button type="button" className="secondary-btn" onClick={onClose} disabled={busy}>Cancel</button><button className="primary-btn" type="submit" disabled={busy}>{busy ? <><LoaderCircle className="spin" size={15} /> Saving…</> : editing ? 'Save changes' : 'Create task'} <ArrowRight size={15} /></button></div></form></ModalFrame>
}

function ProjectForm({ initial, onClose, onSubmit }) {
  const [form, setForm] = useState({ ...initial })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async (event) => { event.preventDefault(); if (!form.name.trim()) return setError('Project name is required.'); if (form.name.trim().length > 80) return setError('Project names can be up to 80 characters.'); setBusy(true); try { await onSubmit({ ...form, name: form.name.trim(), description: form.description.trim() }) } catch { setError('Could not save project. Try again from the notification.') } finally { setBusy(false) } }
  useEscape(onClose)
  return <ModalFrame title={initial.id ? 'Edit project' : 'Create a project'} onClose={onClose}><form className="modal-form" onSubmit={submit}><label className="form-field">Project name<input autoFocus maxLength="80" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="e.g. Semester Project" /></label><label className="form-field">Description<textarea rows="3" maxLength="1000" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="What belongs in this project?" /></label>{error && <div className="form-error" role="alert">{error}</div>}<div className="modal-actions"><button type="button" className="secondary-btn" onClick={onClose}>Cancel</button><button className="primary-btn" disabled={busy}>{busy ? 'Saving…' : initial.id ? 'Save project' : 'Create project'}</button></div></form></ModalFrame>
}

function ModalFrame({ title, eyebrow, onClose, children }) {
  useEscape(onClose)
  return <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><section className="modal" role="dialog" aria-modal="true" aria-label={title}><div className="modal-head"><div>{eyebrow && <div className="eyebrow muted-eyebrow">{eyebrow}</div>}<h2>{title}</h2></div><button className="close-modal" onClick={onClose} aria-label="Close dialog"><X size={18} /></button></div>{children}</section></div>
}

function useEscape(onClose) { useEffect(() => { const handler = (event) => event.key === 'Escape' && onClose(); window.addEventListener('keydown', handler); return () => window.removeEventListener('keydown', handler) }, [onClose]) }
function Detail({ label, value }) { return <div className="detail-cell"><span>{label}</span><strong>{value}</strong></div> }
function PanelHeading({ title, subtitle, action, onAction }) { return <div className="panel-heading"><div><h2>{title}</h2>{subtitle && <p>{subtitle}</p>}</div>{action && <button className="panel-action" onClick={onAction}>{action}<ArrowRight size={13} /></button>}</div> }
function StatusBadge({ status }) { return <span className={`status-badge status-${status}`}><span />{statusLabels[status] || status}</span> }
function PriorityBadge({ priority }) { return <span className={`priority-badge priority-${priority}`}><span />{priorityLabels[priority]}</span> }
function MiniTaskList({ tasks, onOpenTask }) { return <div className="mini-task-list">{tasks.map((task) => <button className="mini-task" key={task.id} onClick={() => onOpenTask(task)}><span className="mini-task-dot" /><span><strong>{task.title}</strong><small>{task.project_name || statusLabels[task.status]}</small></span><time>{prettyDate(task.due_date)}</time></button>)}</div> }
function WeeklyChart({ data }) { const max = Math.max(1, ...data.map((item) => item.count)); return <div className="week-chart">{data.map((item) => <div className="week-bar" key={item.date}><span>{item.count || ''}</span><i style={{ height: `${Math.max(item.count ? 12 : 4, item.count / max * 100)}%` }} /><small>{item.label}</small></div>)}</div> }
function InlineEmpty({ text }) { return <div className="inline-empty"><span className="empty-icon"><Sparkles size={17} /></span><p>{text}</p></div> }
function EmptyState({ title, detail, action, actionLabel }) { return <div className="state-card empty-state"><div className="empty-icon"><ListTodo size={22} /></div><h3>{title}</h3><p>{detail}</p>{action && <button className="secondary-btn" onClick={action}>{actionLabel}</button>}</div> }
function Skeleton({ className = '' }) { return <span className={`skeleton ${className}`} /> }

export default App
