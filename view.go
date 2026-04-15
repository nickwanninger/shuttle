package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.fatalErr != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.fatalErr)
	}
	return m.renderBoard() + "\n" + m.renderBar()
}

func (m model) renderBar() string {
	var status string
	if m.loading && len(m.routeData) == 0 {
		status = m.spinner.View()
	} else if !m.lastRefresh.IsZero() {
		next := refreshInterval - m.tickCount
		status = dimStyle.Render(fmt.Sprintf("next refresh in %ds", next))
	}

	if m.nicknaming {
		stopName := m.stopNames[m.nicknamingStopID]
		return dimStyle.Render("enter/esc  nickname for "+stopName+":  ") + m.nicknameInput.View() + "\n"
	}
	if m.searching {
		return dimStyle.Render("esc to clear  ") + m.searchInput.View() + "  " + status + "\n"
	}
	return dimStyle.Render("/ search  j/k move  f fav  n nickname  a abs/rel  r refresh  q quit  ") + status + "\n"
}

func (m model) renderBoard() string {
	if len(m.routes) == 0 {
		return fmt.Sprintf("\n  %s  Connecting…\n", m.spinner.View())
	}

	var sb strings.Builder
	query := strings.ToLower(m.searchInput.Value())
	now := time.Now()
	flatIdx := 0

	// ── Favorites section (hidden while searching) ─────────────────────────────
	if query == "" && len(m.favorites) > 0 {
		favSeen := make(map[string]bool)
		var favStops []StopWithArrivals
		for _, route := range m.routes {
			rides, loaded := m.routeData[route.RouteID]
			if !loaded {
				continue
			}
			for _, s := range buildStops(rides, m.stopNames, m.favorites) {
				if _, isFav := m.favorites[s.StopID]; isFav && !favSeen[s.StopID] {
					favSeen[s.StopID] = true
					// Apply nickname as the display name
					if nick := m.favorites[s.StopID]; nick != "" {
						s.Name = nick
					}
					favStops = append(favStops, s)
				}
			}
		}
		if len(favStops) > 0 {
			sb.WriteString(fmt.Sprintf("\n  %s\n", directionStyle.Render("Favorites")))
			for _, stop := range favStops {
				selected := flatIdx == m.selectedStop
				sb.WriteString(renderStopLine(stop, now, true, selected, m.relativeTime))
				flatIdx++
			}
		}
	}

	// ── Route sections ─────────────────────────────────────────────────────────
	// When not searching, omit stops that are already shown in the favorites section.
	for _, route := range m.routes {
		rides, loaded := m.routeData[route.RouteID]
		if !loaded {
			sb.WriteString(fmt.Sprintf("\n  %s  %s\n",
				m.spinner.View(),
				dimStyle.Render(strings.TrimPrefix(route.Name, "Intercampus-")),
			))
			continue
		}

		stops := buildStops(rides, m.stopNames, m.favorites)

		// Filter by search query if active
		var filtered []StopWithArrivals
		for _, s := range stops {
			if query == "" || strings.Contains(strings.ToLower(s.Name), query) {
				filtered = append(filtered, s)
			}
		}

		if len(filtered) == 0 {
			continue
		}

		label := strings.TrimPrefix(route.Name, "Intercampus-")
		sb.WriteString(fmt.Sprintf("\n  %s\n", directionStyle.Render(label)))

		for _, stop := range filtered {
			selected := flatIdx == m.selectedStop
			sb.WriteString(renderStopLine(stop, now, false, selected, m.relativeTime))
			flatIdx++
		}
	}

	return sb.String()
}

// buildStops computes the ordered list of StopWithArrivals for a set of rides.
func buildStops(rides []Ride, stopNames map[string]string, favorites map[string]string) []StopWithArrivals {
	type arrivalsByStop = map[string][]ArrivalEntry

	arrivalsMap := make(arrivalsByStop)
	stopOrder := []string{} // preserve first-seen order
	seen := make(map[string]bool)

	for _, ride := range rides {
		if _, completed := ride.State["Completed"]; completed {
			continue
		}
		_, isActive := ride.State["Active"]

		for _, ss := range ride.StopStatus {
			info, ok := ss["Awaiting"]
			if !ok || info.StopID == "" || info.ExpectedArrivalTime == "" {
				continue
			}
			eta, err := time.Parse(time.RFC3339, info.ExpectedArrivalTime)
			if err != nil {
				continue
			}
			sid := info.StopID
			if !seen[sid] {
				seen[sid] = true
				stopOrder = append(stopOrder, sid)
			}
			arrivalsMap[sid] = append(arrivalsMap[sid], ArrivalEntry{
				ETA:      eta,
				IsActive: isActive,
				LateSec:  ride.LateBySec,
			})
		}
	}

	// Sort arrivals per stop by ETA, cap at 4
	stops := make([]StopWithArrivals, 0, len(stopOrder))
	for _, sid := range stopOrder {
		arrivals := arrivalsMap[sid]
		sort.Slice(arrivals, func(i, j int) bool {
			return arrivals[i].ETA.Before(arrivals[j].ETA)
		})
		if len(arrivals) > 4 {
			arrivals = arrivals[:4]
		}
		name := sid[:8] + "…"
		if n, ok := stopNames[sid]; ok {
			name = n
		}
		stops = append(stops, StopWithArrivals{
			StopID:   sid,
			Name:     name,
			Arrivals: arrivals,
		})
	}

	return stops
}

const nameColWidth = 28 // visible characters for the stop name column

// renderStopLine renders one stop as a single line:
//   > ★ Stop Name       ● 10:45 +2m   ·  ○ 11:20      ·  ○ 12:00
func renderStopLine(stop StopWithArrivals, now time.Time, isFav, selected, relativeTime bool) string {
	// Cursor gutter: 2 chars
	var cursor string
	if selected {
		cursor = cursorStyle.Render("> ")
	} else {
		cursor = "  "
	}

	// Favorite slot: 2 chars
	var star string
	if isFav {
		star = favoriteStyle.Render("★ ")
	} else {
		star = "  "
	}

	// Pad name to nameColWidth visible chars — use lipgloss.Width so ANSI
	// bold escapes from stopNameStyle don't break the column width.
	padded := stop.Name + strings.Repeat(" ", nameColWidth-lipgloss.Width(stop.Name))
	var name string
	if selected {
		name = selectedStyle.Render(padded)
	} else {
		name = stopNameStyle.Render(padded)
	}

	var chips []string
	for _, a := range stop.Arrivals {
		chips = append(chips, renderArrivalChip(a, now, relativeTime))
	}
	return cursor + star + name + "  " + strings.Join(chips, dimStyle.Render("  ·  ")) + "\n"
}

func renderArrivalChip(a ArrivalEntry, now time.Time, relativeTime bool) string {
	mins := a.ETA.Sub(now).Minutes()
	var t string
	if relativeTime {
		m := int(mins)
		if m < 0 {
			t = fmt.Sprintf("%-8s", fmt.Sprintf("%dm ago", -m))
		} else {
			t = fmt.Sprintf("%-8s", fmt.Sprintf("in %dm", m))
		}
	} else {
		t = a.ETA.Local().Format("3:04") // always 5 visible chars
	}

	// Late tag: always 4 visible chars wide ("+10m", "+2m ", "    ")
	var lateTag string
	if a.LateSec > 90 {
		lateTag = lateStyle.Render(fmt.Sprintf("%-4s", fmt.Sprintf("+%dm", a.LateSec/60)))
	} else {
		lateTag = "    "
	}

	var dot, timeStr string
	switch {
	case mins < 0:
		dot = dimStyle.Render("·")
		timeStr = dimStyle.Render(t)
	case mins < 5:
		dot = redBoldStyle.Render("●")
		timeStr = redBoldStyle.Render(t)
	case mins < 15:
		dot = yellowStyle.Render("●")
		timeStr = yellowStyle.Render(t)
	default:
		if a.IsActive {
			dot = dimStyle.Render("●")
		} else {
			dot = dimStyle.Render("○")
		}
		timeStr = dimStyle.Render(t)
	}
	return dot + " " + timeStr + " " + lateTag
}
