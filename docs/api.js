// api.js — translates api.go
// Fetches data from Northwestern's TripShot transit API.

const BASE_URL = 'https://northwestern.tripshot.com';
const GROUP_ID = '2ca5dc76-dd3f-4ab4-bd10-056785a989ed';
const HEADERS = {
  'Accept': '*/*',
  'Referer': BASE_URL + '/g/tms/Public.html',
};

async function apiGet(url) {
  const resp = await fetch(url, { headers: HEADERS });
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status} for ${url}`);
  }
  return resp.json();
}

// Returns [{routeId, name}, ...]
export async function fetchRouteList() {
  const url = `${BASE_URL}/v1/p/route?routeGroupId=${GROUP_ID}`;
  const proxy_url = `https://api.allorigins.win/raw?url=${encodeURIComponent(url)}`;
  return apiGet(proxy_url);
}

// Returns {rides: [...]} for the given routeId on today's date.
export async function fetchRouteSummary(routeId) {
  const today = new Date().toISOString().slice(0, 10); // YYYY-MM-DD
  const url = `${BASE_URL}/v2/p/routeSummary/${routeId}?day=${today}`;
  const proxy_url = `https://api.allorigins.win/raw?url=${encodeURIComponent(url)}`;
  const data = await apiGet(proxy_url);
  return data.rides ?? [];
}

// Extracts stopId → name from the vias embedded in each ride.
// Mirrors extractStopNames() in api.go.
export function extractStopNames(rides) {
  const names = new Map();
  for (const ride of rides) {
    for (const viaMap of (ride.vias ?? [])) {
      // Each viaMap is {<stopId>: {stop: {stopId, name}}}
      const info = Object.values(viaMap)[0];
      if (info?.stop?.stopId && info?.stop?.name) {
        names.set(info.stop.stopId, info.stop.name);
      }
    }
  }
  return names;
}
