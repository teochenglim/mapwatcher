/**
 * MapWatch — core client-side JS
 * Handles: WebSocket, marker management, clustering, side panel, themes.
 */
(function () {
  'use strict';

  // ── Constants ────────────────────────────────────────────────────────────────

  const SEVERITY_COLORS = {
    critical: '#f85149',
    warning:  '#e3b341',
    info:     '#58a6ff',
    unknown:  '#8b949e',
  };

  const THEMES = {
    dark: {
      url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
      attribution: '&copy; CartoDB',
    },
    light: {
      url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
      attribution: '&copy; CartoDB',
    },
    satellite: {
      url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
      attribution: '&copy; Esri',
    },
  };

  // ── State ─────────────────────────────────────────────────────────────────────

  let map;
  let tileLayer;
  let markerMap    = {};  // id → { leafletMarker, data }
  let geoBoundsMap = {};  // id → L.Rectangle (hover overlay)
  let clusterGroup;       // L.markerClusterGroup (cluster mode)
  let spreadLayer;        // L.LayerGroup (spread mode)
  let clusterMode  = true;
  let bordersLayer = null;
  let bordersVisible = true;
  let currentTheme = 'dark';
  let activeMarkerId = null;
  let ws;
  let wsReconnectTimer;
  let effects = {};
  // Prometheus external URL fetched from /api/config on startup.
  let prometheusURL = 'http://localhost:9090';

  // ── Initialise map ────────────────────────────────────────────────────────────

  // Singapore bounding box — matches geo/slice.go RegionBounds["SG"]
  // [minLat, minLng, maxLat, maxLng] in Leaflet order
  const SG_BOUNDS = [[1.159, 103.605], [1.482, 104.088]];

  function init() {
    // All user interaction disabled — the map is a fixed viewport of Singapore.
    map = L.map('map', {
      zoomControl:       false,
      dragging:          false,
      touchZoom:         false,
      scrollWheelZoom:   false,
      doubleClickZoom:   false,
      boxZoom:           false,
      keyboard:          false,
      preferCanvas:      true,
    }).fitBounds(SG_BOUNDS, { padding: [20, 20] });

    tileLayer = L.tileLayer(THEMES.dark.url, {
      attribution: THEMES.dark.attribution,
      subdomains: 'abcd',
      maxZoom: 19,
    }).addTo(map);

    clusterGroup = L.markerClusterGroup({ chunkedLoading: true });
    spreadLayer  = L.layerGroup();

    clusterGroup.addTo(map);

    // Keyboard shortcut: Esc closes panel.
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') MapWatch.closePanel();
    });

    // Fetch runtime config (Prometheus external URL, etc.) from server.
    fetchConfig();
    connectWS();
  }

  // ── Runtime config ────────────────────────────────────────────────────────────

  function fetchConfig() {
    fetch('/api/config')
      .then(r => r.json())
      .then(cfg => {
        if (cfg && cfg.prometheusUrl) prometheusURL = cfg.prometheusUrl;
      })
      .catch(() => { /* use default */ });
  }

  // ── WebSocket ─────────────────────────────────────────────────────────────────

  function connectWS() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url   = `${proto}://${location.host}/ws`;
    ws = new WebSocket(url);

    ws.onopen = () => {
      document.getElementById('ws-status').classList.add('connected');
      clearTimeout(wsReconnectTimer);
    };

    ws.onclose = () => {
      document.getElementById('ws-status').classList.remove('connected');
      wsReconnectTimer = setTimeout(connectWS, 3000);
    };

    ws.onerror = (err) => {
      console.error('WS error', err);
      ws.close();
    };

    ws.onmessage = (evt) => {
      let msg;
      try { msg = JSON.parse(evt.data); } catch { return; }
      handleWSEvent(msg);
    };
  }

  function handleWSEvent(msg) {
    switch (msg.type) {
      case 'marker.add':    upsertMarker(msg.marker, true);  break;
      case 'marker.update': upsertMarker(msg.marker, false); break;
      case 'marker.remove': removeMarker(msg.id);            break;
    }
    runEffects(msg);
  }

  // ── Marker management ─────────────────────────────────────────────────────────

  function effectiveLat(m) {
    return m.lat + (m.offset ? m.offset.Lat : 0);
  }

  function effectiveLng(m) {
    return m.lng + (m.offset ? m.offset.Lng : 0);
  }

  function makeIcon(data) {
    const color = severityColor(data.severity);
    const pulse = (data.severity === 'critical') ? 'mw-pulse' : '';
    return L.divIcon({
      className: '',
      html: `<div class="mw-marker ${pulse}" style="background:${color};border-color:${color}"></div>`,
      iconSize: [20, 20],
      iconAnchor: [10, 10],
      tooltipAnchor: [10, 0],
    });
  }

  function makeTooltipHtml(d) {
    const sev   = d.severity || 'unknown';
    const color = severityColor(sev);
    const instance = (d.labels && d.labels.instance) || '';
    const dc       = (d.labels && d.labels.datacenter) || '';
    const summary  = (d.annotations && d.annotations.summary) || '';

    let html = `<div class="mw-tt">
      <div class="mw-tt-title">${e(d.alertname || d.id)}</div>
      <div class="mw-tt-sev" style="color:${color}">${sev.toUpperCase()}</div>`;

    if (instance) html += `<div class="mw-tt-row">${e(instance)}</div>`;
    if (dc)       html += `<div class="mw-tt-row">DC: ${e(dc)}</div>`;
    if (summary)  html += `<div class="mw-tt-row">${e(summary)}</div>`;

    html += `<div class="mw-tt-hint">Click to view in Prometheus ↗</div></div>`;
    return html;
  }

  function upsertMarker(data, isNew) {
    if (markerMap[data.id]) {
      // Update existing
      const { leafletMarker } = markerMap[data.id];
      const lat = effectiveLat(data);
      const lng = effectiveLng(data);
      leafletMarker.setLatLng([lat, lng]);
      leafletMarker.setIcon(makeIcon(data));
      // Refresh tooltip content with latest data.
      leafletMarker.unbindTooltip();
      leafletMarker.bindTooltip(makeTooltipHtml(data), {
        permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
      });
      markerMap[data.id].data = data;
    } else {
      // Create new
      const lat = effectiveLat(data);
      const lng = effectiveLng(data);

      const lm = L.marker([lat, lng], { icon: makeIcon(data) });

      lm.bindTooltip(makeTooltipHtml(data), {
        permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
      });
      lm.on('click', () => MapWatch.openPanel(data.id));

      markerMap[data.id] = { leafletMarker: lm, data };
      addToActiveLayer(lm);
    }
  }

  function removeMarker(id) {
    if (!markerMap[id]) return;
    const { leafletMarker } = markerMap[id];
    clusterGroup.removeLayer(leafletMarker);
    spreadLayer.removeLayer(leafletMarker);
    delete markerMap[id];
    if (geoBoundsMap[id]) {
      map.removeLayer(geoBoundsMap[id]);
      delete geoBoundsMap[id];
    }
    if (activeMarkerId === id) MapWatch.closePanel();
  }

  function addToActiveLayer(lm) {
    if (clusterMode) {
      clusterGroup.addLayer(lm);
    } else {
      spreadLayer.addLayer(lm);
    }
  }

  function severityColor(sev) {
    return SEVERITY_COLORS[sev] || SEVERITY_COLORS.unknown;
  }

  // ── Effects plugin system ─────────────────────────────────────────────────────

  function registerEffect(name, fn) {
    effects[name] = fn;
  }

  function runEffects(event) {
    for (const fn of Object.values(effects)) {
      try { fn(event, map, markerMap); } catch (err) {
        console.error('effect error:', err);
      }
    }
  }

  // ── Cluster / Spread toggle ───────────────────────────────────────────────────

  function toggleClusterMode() {
    if (clusterMode) return;
    clusterMode = true;

    // Move all markers from spread → cluster
    spreadLayer.clearLayers();
    map.removeLayer(spreadLayer);
    for (const { leafletMarker } of Object.values(markerMap)) {
      clusterGroup.addLayer(leafletMarker);
    }
    clusterGroup.addTo(map);

    document.getElementById('btn-cluster').classList.add('active');
    document.getElementById('btn-spread').classList.remove('active');
  }

  function toggleSpreadMode() {
    if (!clusterMode) return;
    clusterMode = false;

    clusterGroup.clearLayers();
    map.removeLayer(clusterGroup);
    for (const { leafletMarker } of Object.values(markerMap)) {
      spreadLayer.addLayer(leafletMarker);
    }
    spreadLayer.addTo(map);

    document.getElementById('btn-spread').classList.add('active');
    document.getElementById('btn-cluster').classList.remove('active');
  }

  // ── Theme switcher ────────────────────────────────────────────────────────────

  function setTheme(name) {
    const t = THEMES[name];
    if (!t) return;
    currentTheme = name;
    tileLayer.setUrl(t.url);
    ['dark', 'light', 'satellite'].forEach((n) => {
      document.getElementById('btn-' + n).classList.toggle('active', n === name);
    });
  }

  // ── Borders toggle ────────────────────────────────────────────────────────────

  function toggleBorders() {
    bordersVisible = !bordersVisible;
    if (bordersLayer) {
      if (bordersVisible) {
        bordersLayer.addTo(map);
      } else {
        map.removeLayer(bordersLayer);
      }
    }
    document.getElementById('btn-borders').classList.toggle('active', bordersVisible);
  }

  // ── Side panel ────────────────────────────────────────────────────────────────

  function openPanel(id) {
    const entry = markerMap[id];
    if (!entry) return;
    activeMarkerId = id;
    renderPanel(entry.data);
    document.getElementById('panel').classList.add('open');
    loadLinks(id);
  }

  function closePanel() {
    document.getElementById('panel').classList.remove('open');
    activeMarkerId = null;
  }

  function renderPanel(m) {
    const dur = m.startsAt ? timeSince(new Date(m.startsAt)) : '—';
    const sev = m.severity || 'unknown';
    const badgeClass = 'sev-' + (SEVERITY_COLORS[sev] ? sev : 'unknown');

    document.getElementById('panel-title').textContent = m.alertname || m.id;

    const content = document.getElementById('panel-content');
    content.innerHTML = `
      <span class="severity-badge ${badgeClass}">${sev}</span>

      <div class="meta-row"><strong>Instance:</strong> ${e(m.labels && m.labels.instance || '—')}</div>
      <div class="meta-row"><strong>Datacenter:</strong> ${e(m.labels && m.labels.datacenter || '—')}</div>
      <div class="meta-row"><strong>Duration:</strong> ${e(dur)}</div>
      ${m.annotations && m.annotations.summary ? `<div class="meta-row"><strong>Summary:</strong> ${e(m.annotations.summary)}</div>` : ''}
      ${m.annotations && m.annotations.description ? `<div class="meta-row"><strong>Description:</strong> ${e(m.annotations.description)}</div>` : ''}

      <div class="panel-section">
        <h3>Prometheus Metrics</h3>
        <div id="links-loading">Loading…</div>
        <div id="links-error"></div>
        <div id="links-container"></div>
      </div>

      <div class="panel-section">
        <details>
          <summary>Raw labels</summary>
          <div class="labels-grid" style="margin-top:8px">
            ${Object.entries(m.labels || {}).map(([k, v]) =>
              `<span class="label-key">${e(k)}</span><span class="label-value">${e(v)}</span>`
            ).join('')}
          </div>
        </details>
      </div>
    `;
  }

  function loadLinks(id) {
    const loading  = document.getElementById('links-loading');
    const errEl    = document.getElementById('links-error');
    const container = document.getElementById('links-container');

    if (!container) return;
    container.innerHTML = '';
    if (errEl)   { errEl.style.display = 'none'; errEl.textContent = ''; }
    if (loading) loading.style.display = 'block';

    fetch(`/api/markers/${encodeURIComponent(id)}/links`)
      .then(r => r.json())
      .then(links => {
        if (loading) loading.style.display = 'none';
        if (!Array.isArray(links) || links.length === 0) {
          if (errEl) { errEl.textContent = 'No metrics configured for this alert.'; errEl.style.display = 'block'; }
          return;
        }
        container.innerHTML = links.map(l =>
          `<a class="prom-link" href="${e(l.url)}" target="_blank" rel="noopener noreferrer">
            <span>${e(l.label)}</span>
            <span class="prom-link-icon">↗</span>
          </a>`
        ).join('');
      })
      .catch(err => {
        if (loading) loading.style.display = 'none';
        if (errEl)   { errEl.textContent = 'Failed to load links: ' + err.message; errEl.style.display = 'block'; }
      });
  }

  function timeSince(date) {
    const secs = Math.floor((Date.now() - date) / 1000);
    if (secs < 60)   return secs + 's';
    if (secs < 3600) return Math.floor(secs / 60) + 'm';
    if (secs < 86400) return Math.floor(secs / 3600) + 'h';
    return Math.floor(secs / 86400) + 'd';
  }

  // HTML-escape helper
  function e(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // ── Public API ────────────────────────────────────────────────────────────────

  window.MapWatch = {
    registerEffect,
    toggleClusterMode,
    toggleSpreadMode,
    setTheme,
    toggleBorders,
    openPanel,
    closePanel,
    // Exposed for static export mode (pre-load markers without WS)
    loadStaticMarkers(markers) {
      for (const m of markers) upsertMarker(m, true);
    },
  };

  // Boot
  document.addEventListener('DOMContentLoaded', init);
})();
