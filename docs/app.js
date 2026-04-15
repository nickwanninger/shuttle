// app.js — main entry point.
// Orchestrates data fetching, rendering, refresh timer, and event wiring.

import { fetchRouteList, fetchRouteSummary, extractStopNames } from './api.js';
import {
  buildStops,
  loadFavorites,
  saveFavorites,
  toggleFavorite,
  setNickname,
} from './state.js';

// ── App state ─────────────────────────────────────────────────────────────────

const state = {
  routes: [],
  routeData: new Map(),    // routeId → rides[]
  stopNames: new Map(),    // stopId → name
  favorites: loadFavorites(),
  relativeTime: localStorage.getItem('shuttle-relative-time') === '1', // default false
  searchQuery: '',
  loading: true,
  error: null,
  lastRefresh: null,
  countdown: 30,
  expandedStopId: null,    // stopId whose full schedule is shown
  timers: new Map(),       // eta.getTime() → setTimeout ID
};

const REFRESH_INTERVAL = 30; // seconds

// ── Bootstrap ─────────────────────────────────────────────────────────────────

async function init() {
  try {
    state.routes = await fetchRouteList();
  } catch (err) {
    state.error = `Failed to load routes: ${err.message}`;
    state.loading = false;
    render();
    return;
  }
  await refreshAllRoutes();
  wireEvents();
  startCountdown();
}

// ── Data refresh ──────────────────────────────────────────────────────────────

async function refreshAllRoutes() {
  state.loading = true;
  render();

  await Promise.all(state.routes.map(async (route) => {
    try {
      const rides = await fetchRouteSummary(route.routeId);
      state.routeData.set(route.routeId, rides);
      // Merge stop names discovered from this route's rides
      for (const [sid, name] of extractStopNames(rides)) {
        state.stopNames.set(sid, name);
      }
    } catch {
      // Leave stale data in place; the route will still render if we have prior data
    }
  }));

  state.loading = false;
  state.lastRefresh = new Date();
  state.countdown = REFRESH_INTERVAL;
  render();
}

// ── Countdown timer ───────────────────────────────────────────────────────────

function startCountdown() {
  setInterval(() => {
    state.countdown = Math.max(0, state.countdown - 1);
    updateCountdownDisplay();
    if (state.countdown === 0) {
      refreshAllRoutes();
    }
  }, 1000);
}

function updateCountdownDisplay() {
  const el = document.getElementById('countdown');
  if (el) {
    el.textContent = state.lastRefresh
      ? `next refresh in ${state.countdown}s`
      : '';
  }
}

// ── Render ────────────────────────────────────────────────────────────────────

function render() {
  const app = document.getElementById('app');
  if (!app) return;

  if (state.error) {
    app.innerHTML = `<p class="error-msg">${escHtml(state.error)}</p>`;
    return;
  }

  if (state.loading && state.routeData.size === 0) {
    app.innerHTML = `<p class="loading-msg"><span class="spinner">⠋</span> Connecting…</p>`;
    return;
  }

  const query = state.searchQuery.trim().toLowerCase();
  const now = new Date();
  let html = '';

  // Favorites section (hidden while searching)
  if (!query && state.favorites.size > 0) {
    const favStops = [];
    const favSeen = new Set();
    for (const route of state.routes) {
      const rides = state.routeData.get(route.routeId) ?? [];
      for (const stop of buildStops(rides, state.stopNames, state.favorites)) {
        if (state.favorites.has(stop.stopId) && !favSeen.has(stop.stopId)) {
          favSeen.add(stop.stopId);
          const nick = state.favorites.get(stop.stopId);
          favStops.push({ ...stop, name: nick || stop.name });
        }
      }
    }
    if (favStops.length > 0) {
      html += renderSection('Favorites', favStops, now, true);
    }
  }

  // Route sections
  for (const route of state.routes) {
    const rides = state.routeData.get(route.routeId) ?? [];
    const stops = buildStops(rides, state.stopNames, state.favorites);
    const filtered = query
      ? stops.filter(s => s.name.toLowerCase().includes(query))
      : stops;

    if (filtered.length === 0) continue;

    const label = route.name.replace(/^Intercampus-/, '');
    const isLoading = state.loading && rides.length === 0;
    html += renderSection(label, filtered, now, false, isLoading);
  }

  if (!html) {
    html = query
      ? `<p class="loading-msg" style="color:var(--color-dim)">No stops match "${escHtml(query)}".</p>`
      : `<p class="loading-msg" style="color:var(--color-dim)">No active routes right now.</p>`;
  }

  app.innerHTML = html;
  rewireStopButtons();
  updateCountdownDisplay();
}

function renderSection(label, stops, now, isFavSection, isLoading = false) {
  const spinner = isLoading ? '<span class="spinner">⠋</span> ' : '';
  let html = `<div class="route-section">`;
  html += `<div class="route-header">${spinner}${escHtml(label)}</div>`;
  for (const stop of stops) {
    html += renderStopRow(stop, now, isFavSection);
  }
  html += `</div>`;
  return html;
}

function renderStopRow(stop, now, isFav) {
  const isFavStop = state.favorites.has(stop.stopId);
  const isExpanded = state.expandedStopId === stop.stopId;
  const starClass = isFavStop ? 'fav-btn is-fav' : 'fav-btn';
  const starChar = isFavStop ? '★' : '☆';

  const chipsHtml = stop.arrivals.map((a, i) => {
    const extra = i > 0 ? ' chip-extra' : '';
    const sep = i > 0 ? `<span class="chip-sep chip-extra">·</span>` : '';
    const primary = i === 0 && window.innerWidth > 640;
    return sep + renderChip(a, now, extra, primary);
  }).join('');

  // Favorited names open the nickname editor; all other names toggle the expanded schedule
  const nameEl = state.nicknamingStopId === stop.stopId
    ? `<input class="nick-input" id="nick-input" type="text" value="${escAttr(state.favorites.get(stop.stopId) ?? '')}" placeholder="${escAttr(stop.name)}" maxlength="40" autofocus>`
    : isFavStop
      ? `<span class="stop-name fav-name" data-action="nickname" data-stop-id="${escAttr(stop.stopId)}" title="Click to set nickname">${escHtml(stop.name)}</span>`
      : `<span class="stop-name expandable${isExpanded ? ' is-expanded' : ''}" data-action="expand" data-stop-id="${escAttr(stop.stopId)}">${escHtml(stop.name)}</span>`;

  const expandedHtml = isExpanded ? renderExpandedSchedule(stop.stopId, now) : '';

  return `<div class="stop-row${isExpanded ? ' is-expanded' : ''}" data-stop-id="${escAttr(stop.stopId)}">` +
    `<div class="stop-row-main">` +
    `<button class="${starClass}" data-action="fav" data-stop-id="${escAttr(stop.stopId)}" aria-label="${isFavStop ? 'unfavorite' : 'favorite'}">${starChar}</button>` +
    nameEl +
    `<div class="arrivals">${chipsHtml}</div>` +
    `</div>` +
    expandedHtml +
    `</div>`;
}

// Returns all future arrivals for a stop across all routes, uncapped, sorted by ETA.
function getAllArrivalsForStop(stopId) {
  const arrivals = [];
  for (const rides of state.routeData.values()) {
    for (const ride of rides) {
      if ('Completed' in (ride.state ?? {})) continue;
      const isActive = 'Active' in (ride.state ?? {});
      for (const ssMap of (ride.stopStatus ?? [])) {
        const info = ssMap['Awaiting'];
        if (info?.stopId !== stopId || !info?.expectedArrivalTime) continue;
        const eta = new Date(info.expectedArrivalTime);
        if (isNaN(eta.getTime())) continue;
        arrivals.push({ eta, isActive, lateSec: ride.lateBySec ?? 0 });
      }
    }
  }
  arrivals.sort((a, b) => a.eta - b.eta);
  return arrivals;
}

function renderExpandedSchedule(stopId, now) {
  const arrivals = getAllArrivalsForStop(stopId);
  if (arrivals.length === 0) {
    return `<div class="stop-expanded"><span class="dim">No more departures today.</span></div>`;
  }
  const chipsHtml = arrivals.map((a, i) => {
    const sep = i > 0 ? `<span class="chip-sep">·</span>` : '';
    return sep + renderChip(a, now);
  }).join('');
  return `<div class="stop-expanded">${chipsHtml}</div>`;
}

// Translates renderArrivalChip() from view.go:238
// primary=true: always show "12:20 PM (in 20m)" regardless of relativeTime toggle.
// primary=false: obey relativeTime toggle (used for extra chips).
function renderChip(arrival, now, extraClass = '', primary = false) {
  const mins = (arrival.eta - now) / 60000;

  let urgency;
  let dotChar;
  if (mins < 0) {
    urgency = 'past';
    dotChar = '·';
  } else if (mins < 5) {
    urgency = 'imminent';
    dotChar = '●';
  } else if (mins < 15) {
    urgency = 'near';
    dotChar = '●';
  } else {
    urgency = 'far';
    dotChar = arrival.isActive ? '●' : '○';
  }

  let timeStr;
  if (primary) {
    const abs = arrival.eta.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    const m = Math.round(mins);
    const rel = m < 0 ? `${-m}m ago` : `in ${m}m`;
    timeStr = `${abs} (${rel})`;
  } else if (state.relativeTime) {
    const m = Math.round(mins);
    timeStr = m < 0 ? `${-m}m ago` : `in ${m}m`;
  } else {
    timeStr = arrival.eta.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
  }

  let lateHtml = '';
  if (arrival.lateSec > 90) {
    const lateMin = Math.floor(arrival.lateSec / 60);
    lateHtml = `<span class="chip-late">+${lateMin}m</span>`;
  }

  const etaKey = arrival.eta.getTime();
  const hasTimer = state.timers.has(etaKey);
  const timerHtml = hasTimer ? `<span class="chip-timer-bell">🔔</span>` : '';
  const timerClass = hasTimer ? ' has-timer' : '';

  return `<span class="chip ${urgency}${extraClass}${timerClass}" data-action="timer" data-eta="${etaKey}" style="cursor:pointer">` +
    `<span class="chip-dot">${dotChar}</span>` +
    `<span class="chip-time">${escHtml(timeStr)}</span>` +
    lateHtml +
    timerHtml +
    `</span>`;
}

// ── Event wiring ──────────────────────────────────────────────────────────────

function wireEvents() {
  // Search input
  document.getElementById('search-input').addEventListener('input', (e) => {
    state.searchQuery = e.target.value;
    render();
  });

  // Refresh button
  document.getElementById('btn-refresh').addEventListener('click', () => {
    state.countdown = REFRESH_INTERVAL;
    refreshAllRoutes();
  });

  // Time toggle button
  const timeToggle = document.getElementById('btn-time-toggle');
  const timeLabel = document.getElementById('time-toggle-label');
  // Set initial label to match loaded state
  timeLabel.textContent = state.relativeTime ? ' absolute time' : ' relative time';

  timeToggle.addEventListener('click', () => {
    state.relativeTime = !state.relativeTime;
    localStorage.setItem('shuttle-relative-time', state.relativeTime ? '1' : '0');
    timeLabel.textContent = state.relativeTime ? ' absolute time' : ' relative time';
    render();
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    // Don't interfere when typing in an input
    if (e.target.tagName === 'INPUT') return;

    if (e.key === 'r' || e.key === 'R') {
      state.countdown = REFRESH_INTERVAL;
      refreshAllRoutes();
    } else if (e.key === 'a' || e.key === 'A') {
      state.relativeTime = !state.relativeTime;
      localStorage.setItem('shuttle-relative-time', state.relativeTime ? '1' : '0');
      const label = document.getElementById('time-toggle-label');
      if (label) label.textContent = state.relativeTime ? ' absolute time' : ' relative time';
      render();
    } else if (e.key === '/') {
      e.preventDefault();
      document.getElementById('search-input')?.focus();
    } else if (e.key === 'Escape') {
      const searchInput = document.getElementById('search-input');
      if (searchInput && document.activeElement === searchInput) {
        searchInput.value = '';
        state.searchQuery = '';
        render();
        searchInput.blur();
      }
    }
  });
}

// Re-attaches click handlers to dynamically rendered stop row buttons.
function rewireStopButtons() {
  // Favorite toggle buttons
  // Star button always toggles favorite on/off
  document.querySelectorAll('[data-action="fav"]').forEach((btn) => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const sid = btn.dataset.stopId;
      state.favorites = toggleFavorite(state.favorites, sid);
      saveFavorites(state.favorites);
      if (!state.favorites.has(sid)) state.nicknamingStopId = null;
      render();
    });
  });

  // Stop name click (non-favorite) → toggle expanded schedule
  document.querySelectorAll('[data-action="expand"]').forEach((el) => {
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const sid = el.dataset.stopId;
      state.expandedStopId = state.expandedStopId === sid ? null : sid;
      render();
    });
  });

  // Stop name click on a favorite → open nickname editor
  document.querySelectorAll('[data-action="nickname"]').forEach((el) => {
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const sid = el.dataset.stopId;
      state.nicknamingStopId = state.nicknamingStopId === sid ? null : sid;
      render();
      const inp = document.getElementById('nick-input');
      inp?.focus();
      inp?.select();
    });
  });

  // Nickname input
  const nickInput = document.getElementById('nick-input');
  if (nickInput) {
    nickInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        commitNickname(nickInput.value);
      } else if (e.key === 'Escape') {
        state.nicknamingStopId = null;
        render();
      }
    });
    nickInput.addEventListener('blur', () => {
      // Small delay to let click events fire first
      setTimeout(() => {
        if (state.nicknamingStopId !== null) {
          commitNickname(nickInput.value);
        }
      }, 150);
    });
  }

  // Chip click → set/cancel 10-minute departure timer
  document.querySelectorAll('[data-action="timer"]').forEach((chip) => {
    chip.addEventListener('click', (e) => {
      e.stopPropagation();
      const etaKey = parseInt(chip.dataset.eta, 10);
      setDepartureTimer(new Date(etaKey));
    });
  });
}

function commitNickname(value) {
  if (state.nicknamingStopId) {
    state.favorites = setNickname(state.favorites, state.nicknamingStopId, value);
    saveFavorites(state.favorites);
    state.nicknamingStopId = null;
    render();
  }
}

// ── Departure timers ──────────────────────────────────────────────────────────

async function setDepartureTimer(eta) {
  const etaKey = eta.getTime();

  // Toggle off if already set
  if (state.timers.has(etaKey)) {
    clearTimeout(state.timers.get(etaKey));
    state.timers.delete(etaKey);
    render();
    showToast('Timer cancelled.');
    return;
  }

  const fireAt = new Date(eta - 10 * 60 * 1000);
  const delay = fireAt - Date.now();
  const timeStr = eta.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });

  if (delay <= 0) {
    showToast(`Departure at ${timeStr} is less than 10 minutes away!`);
    return;
  }

  // Request notification permission (requires user gesture — we're in a click handler)
  let canNotify = false;
  if ('Notification' in window) {
    if (Notification.permission === 'granted') {
      canNotify = true;
    } else if (Notification.permission !== 'denied') {
      canNotify = (await Notification.requestPermission()) === 'granted';
    }
  }

  const id = setTimeout(() => {
    state.timers.delete(etaKey);
    render();
    const msg = `Your shuttle departs at ${timeStr} — time to head out!`;
    if (canNotify) {
      new Notification('Shuttle departing in 10 minutes', { body: msg, icon: 'favicon.ico' });
    }
    showToast(msg);
  }, delay);

  state.timers.set(etaKey, id);
  render();
  showToast(`Timer set — you'll be notified 10 min before ${timeStr}.`);
}

let toastTimer = null;

function showToast(msg) {
  let toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('visible');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.classList.remove('visible'), 4000);
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function escHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function escAttr(str) {
  return String(str).replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// ── Start ─────────────────────────────────────────────────────────────────────

init();
