// state.js — favorites persistence and buildStops logic.
// Translates the relevant parts of model.go and view.go (buildStops).

const STORAGE_KEY = 'shuttle-favorites';

// Returns Map<stopId, nickname> loaded from localStorage.
export function loadFavorites() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return new Map();
    const data = JSON.parse(raw); // {favorites: [{stopId, nickname?}]}
    const map = new Map();
    for (const entry of (data.favorites ?? [])) {
      map.set(entry.stopId, entry.nickname ?? '');
    }
    return map;
  } catch {
    return new Map();
  }
}

// Persists favorites Map<stopId, nickname> to localStorage.
export function saveFavorites(map) {
  const favorites = [];
  for (const [stopId, nickname] of map) {
    favorites.push(nickname ? { stopId, nickname } : { stopId });
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ favorites }));
}

// Toggles favorite status for a stop.
export function toggleFavorite(map, stopId) {
  const next = new Map(map);
  if (next.has(stopId)) {
    next.delete(stopId);
  } else {
    next.set(stopId, '');
  }
  return next;
}

// Sets a nickname for a favorited stop (no-op if not a favorite).
export function setNickname(map, stopId, nickname) {
  if (!map.has(stopId)) return map;
  const next = new Map(map);
  next.set(stopId, nickname.trim());
  return next;
}

// buildStops — Returns [{stopId, name, arrivals: [{eta: Date, isActive: bool, lateSec: int}]}]
//
// Stop ORDER comes from ride.vias, which lists all stops in route sequence regardless
// of whether they've been visited. Stop ARRIVALS come from ride.stopStatus (Awaiting
// entries only). This means stops always appear in correct route order even when the
// active ride is mid-route and has already passed some stops.
export function buildStops(rides, stopNames, favorites) {
  // Pass 1: build arrivals map from all non-completed rides' stopStatus.
  const arrivalsMap = new Map(); // stopId → [{eta, isActive, lateSec}]

  for (const ride of rides) {
    if ('Completed' in (ride.state ?? {})) continue;
    const isActive = 'Active' in (ride.state ?? {});

    for (const ssMap of (ride.stopStatus ?? [])) {
      const info = ssMap['Awaiting'];
      if (!info?.stopId || !info?.expectedArrivalTime) continue;

      const eta = new Date(info.expectedArrivalTime);
      if (isNaN(eta.getTime())) continue;

      const sid = info.stopId;
      if (!arrivalsMap.has(sid)) arrivalsMap.set(sid, []);
      arrivalsMap.get(sid).push({ eta, isActive, lateSec: ride.lateBySec ?? 0 });
    }
  }

  // Pass 2: determine canonical stop order from vias of any ride.
  // Vias list every stop on the route in sequence — active, visited, and future.
  // Use the first ride that has vias; all rides on the same route share the same sequence.
  let stopOrder = [];
  for (const ride of rides) {
    if (ride.vias?.length > 0) {
      stopOrder = ride.vias
        .map(viaMap => Object.keys(viaMap)[0])
        .filter(Boolean);
      break;
    }
  }

  // Fallback: if no vias data, use arrival order (preserves old behaviour).
  if (stopOrder.length === 0) {
    stopOrder = [...arrivalsMap.keys()];
  }

  // Pass 3: build result in via order, skipping stops with no arrivals.
  const stops = [];
  const emitted = new Set();

  for (const sid of stopOrder) {
    if (emitted.has(sid)) continue;
    emitted.add(sid);

    const arrivals = arrivalsMap.get(sid);
    if (!arrivals?.length) continue;

    arrivals.sort((a, b) => a.eta - b.eta);

    let name = sid.slice(0, 8) + '…';
    if (stopNames.has(sid)) name = stopNames.get(sid);

    stops.push({ stopId: sid, name, arrivals: arrivals.slice(0, 4) });
  }

  return stops;
}
