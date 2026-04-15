package main

import "time"

// ── API response types ────────────────────────────────────────────────────────

type Route struct {
	RouteID string `json:"routeId"`
	Name    string `json:"name"`
}

type RouteSummary struct {
	Rides []Ride `json:"rides"`
}

type Ride struct {
	RideID         string                 `json:"rideId"`
	ScheduledStart string                 `json:"scheduledStart"`
	State          map[string]interface{} `json:"state"`
	LateBySec      int                    `json:"lateBySec"`
	StopStatus     []map[string]StopInfo  `json:"stopStatus"`
	Vias           []map[string]ViaInfo   `json:"vias"`
}

type StopInfo struct {
	StopID              string `json:"stopId"`
	ExpectedArrivalTime string `json:"expectedArrivalTime"`
}

type ViaInfo struct {
	Stop *StopDetail `json:"stop"`
}

type StopDetail struct {
	StopID string `json:"stopId"`
	Name   string `json:"name"`
}

// ── Persistence types ─────────────────────────────────────────────────────────

// FavoriteEntry stores a favorited stop and its optional nickname.
type FavoriteEntry struct {
	StopID   string `json:"stopId"`
	Nickname string `json:"nickname,omitempty"`
}

type FavoritesData struct {
	Favorites []FavoriteEntry `json:"favorites"`
}

// ── View/domain types (computed during rendering) ─────────────────────────────

type StopWithArrivals struct {
	StopID   string
	Name     string
	Arrivals []ArrivalEntry
}

type ArrivalEntry struct {
	ETA      time.Time
	IsActive bool
	LateSec  int
}
