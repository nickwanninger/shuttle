package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const refreshInterval = 30 // seconds between data refreshes

// ── Msg types ─────────────────────────────────────────────────────────────────

type routeListMsg struct {
	routes []Route
	err    error
}

type routeSummaryMsg struct {
	routeID string
	rides   []Ride
	err     error
}

type tickMsg time.Time

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	routes        []Route
	routeData     map[string][]Ride   // routeID → rides
	stopNames     map[string]string   // stopID → human name
	pendingRoutes map[string]struct{} // routeIDs not yet returned this cycle

	favorites    map[string]string // stopID → nickname (empty string = no nickname)
	selectedStop int

	nicknaming       bool
	nicknamingStopID string
	nicknameInput    textinput.Model

	relativeTime bool

	loading     bool
	lastRefresh time.Time
	tickCount   int // counts up to refreshInterval

	spinner     spinner.Model
	searchInput textinput.Model
	searching   bool

	width, height int
	fatalErr      error
}

func initialModel() model {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))
	ti := textinput.New()
	ti.Placeholder = "filter stops…"
	ti.CharLimit = 64

	ni := textinput.New()
	ni.Placeholder = "nickname…"
	ni.CharLimit = 32

	return model{
		routeData:     make(map[string][]Ride),
		stopNames:     make(map[string]string),
		pendingRoutes: make(map[string]struct{}),
		favorites:     loadFavorites(),
		loading:       true,
		spinner:       sp,
		searchInput:   ti,
		nicknameInput: ni,
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchRouteList(),
	)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case routeListMsg:
		if msg.err != nil {
			m.fatalErr = msg.err
			return m, nil
		}
		m.routes = msg.routes
		cmds := make([]tea.Cmd, 0, len(msg.routes)+1)
		for _, r := range msg.routes {
			m.pendingRoutes[r.RouteID] = struct{}{}
			cmds = append(cmds, fetchRouteSummary(r.RouteID))
		}
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case routeSummaryMsg:
		if msg.err == nil {
			m.routeData[msg.routeID] = msg.rides
			for k, v := range extractStopNames(msg.rides) {
				m.stopNames[k] = v
			}
		}
		delete(m.pendingRoutes, msg.routeID)
		if len(m.pendingRoutes) == 0 {
			m.loading = false
			m.lastRefresh = time.Now()
			// Clamp cursor in case visible stop count shrank after refresh
			stops := computeVisibleStops(m)
			if m.selectedStop >= len(stops) && len(stops) > 0 {
				m.selectedStop = len(stops) - 1
			}
		}

	case tickMsg:
		m.tickCount++
		if m.tickCount >= refreshInterval {
			m.tickCount = 0
			return m, m.doRefresh()
		}
		return m, tickCmd()

	case tea.KeyMsg:
		if m.nicknaming {
			switch msg.Type {
			case tea.KeyEsc:
				m.nicknameInput.SetValue("")
				m.nicknameInput.Blur()
				m.nicknaming = false
				m.nicknamingStopID = ""
				return m, nil
			case tea.KeyEnter:
				nick := strings.TrimSpace(m.nicknameInput.Value())
				m.favorites[m.nicknamingStopID] = nick
				saveFavorites(m.favorites) //nolint:errcheck
				m.nicknameInput.SetValue("")
				m.nicknameInput.Blur()
				m.nicknaming = false
				m.nicknamingStopID = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.nicknameInput, cmd = m.nicknameInput.Update(msg)
				return m, cmd
			}
		}
		if m.searching {
			switch msg.Type {
			case tea.KeyEsc:
				m.searchInput.SetValue("")
				m.searchInput.Blur()
				m.searching = false
				m.selectedStop = 0
				return m, nil
			default:
				prev := m.searchInput.Value()
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				if m.searchInput.Value() != prev {
					m.selectedStop = 0
				}
				return m, cmd
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.searching = true
			cmd := m.searchInput.Focus()
			return m, cmd
		case "r":
			return m, m.doRefresh()
		case "j", "down":
			stops := computeVisibleStops(m)
			if m.selectedStop < len(stops)-1 {
				m.selectedStop++
			}
		case "k", "up":
			if m.selectedStop > 0 {
				m.selectedStop--
			}
		case "f":
			stops := computeVisibleStops(m)
			if len(stops) > 0 {
				sid := stops[m.selectedStop]
				if _, isFav := m.favorites[sid]; isFav {
					delete(m.favorites, sid)
				} else {
					m.favorites[sid] = ""
				}
				saveFavorites(m.favorites) //nolint:errcheck
			}
		case "a":
			m.relativeTime = !m.relativeTime
		case "n":
			stops := computeVisibleStops(m)
			if len(stops) > 0 {
				sid := stops[m.selectedStop]
				if _, isFav := m.favorites[sid]; isFav {
					m.nicknamingStopID = sid
					m.nicknameInput.SetValue(m.favorites[sid])
					m.nicknaming = true
					cmd := m.nicknameInput.Focus()
					return m, cmd
				}
			}
		}
	}

	return m, nil
}

func (m *model) doRefresh() tea.Cmd {
	m.tickCount = 0
	cmds := make([]tea.Cmd, 0, len(m.routes)+1)
	for _, r := range m.routes {
		m.pendingRoutes[r.RouteID] = struct{}{}
		cmds = append(cmds, fetchRouteSummary(r.RouteID))
	}
	cmds = append(cmds, tickCmd())
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ── Favorites persistence ──────────────────────────────────────────────────────

func favoritesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "shuttle", "favorites.json"), nil
}

func loadFavorites() map[string]string {
	path, err := favoritesPath()
	if err != nil {
		return make(map[string]string)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]string)
	}
	var fd FavoritesData
	if err := json.Unmarshal(data, &fd); err != nil {
		return make(map[string]string)
	}
	m := make(map[string]string, len(fd.Favorites))
	for _, e := range fd.Favorites {
		m[e.StopID] = e.Nickname
	}
	return m
}

func saveFavorites(favorites map[string]string) error {
	path, err := favoritesPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	entries := make([]FavoriteEntry, 0, len(favorites))
	for id, nick := range favorites {
		entries = append(entries, FavoriteEntry{StopID: id, Nickname: nick})
	}
	data, err := json.Marshal(FavoritesData{Favorites: entries})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// computeVisibleStops returns stop IDs in the same order renderBoard produces them.
// When not searching, the favorites section comes first (deduplicated), then route
// sections (favorites omitted). When searching, only route sections are shown and
// duplicates are not a concern. Used for cursor key handling.
func computeVisibleStops(m model) []string {
	query := strings.ToLower(m.searchInput.Value())
	var ids []string

	if query == "" && len(m.favorites) > 0 {
		// Favorites section: gather stops that are favorited and have live data
		seen := make(map[string]bool)
		for _, route := range m.routes {
			rides, loaded := m.routeData[route.RouteID]
			if !loaded {
				continue
			}
			stops := buildStops(rides, m.stopNames, m.favorites)
			for _, s := range stops {
				if _, isFav := m.favorites[s.StopID]; isFav && !seen[s.StopID] {
					seen[s.StopID] = true
					ids = append(ids, s.StopID)
				}
			}
		}
	}

	// Route sections: include all stops (favorites appear here too, without nickname)
	for _, route := range m.routes {
		rides, loaded := m.routeData[route.RouteID]
		if !loaded {
			continue
		}
		stops := buildStops(rides, m.stopNames, m.favorites)
		for _, s := range stops {
			if query != "" && !strings.Contains(strings.ToLower(s.Name), query) {
				continue
			}
			ids = append(ids, s.StopID)
		}
	}
	return ids
}
