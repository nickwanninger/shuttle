package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	baseURL = "https://northwestern.tripshot.com"
	groupID = "2ca5dc76-dd3f-4ab4-bd10-056785a989ed"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func apiGet(url string, out interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", baseURL+"/g/tms/Public.html")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func fetchRouteList() tea.Cmd {
	return func() tea.Msg {
		var routes []Route
		err := apiGet(fmt.Sprintf("%s/v1/p/route?routeGroupId=%s", baseURL, groupID), &routes)
		return routeListMsg{routes: routes, err: err}
	}
}

func fetchRouteSummary(routeID string) tea.Cmd {
	return func() tea.Msg {
		today := time.Now().Format("2006-01-02")
		var summary RouteSummary
		err := apiGet(fmt.Sprintf("%s/v2/p/routeSummary/%s?day=%s", baseURL, routeID, today), &summary)
		return routeSummaryMsg{routeID: routeID, rides: summary.Rides, err: err}
	}
}

// extractStopNames pulls stopId → name from the vias embedded in each ride.
func extractStopNames(rides []Ride) map[string]string {
	names := make(map[string]string)
	for _, ride := range rides {
		for _, via := range ride.Vias {
			for _, info := range via {
				if info.Stop != nil && info.Stop.StopID != "" && info.Stop.Name != "" {
					names[info.Stop.StopID] = info.Stop.Name
				}
				break
			}
		}
	}
	return names
}
