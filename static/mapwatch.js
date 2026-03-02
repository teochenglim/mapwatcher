/**
 * MapWatch — core client-side JS
 * Handles: WebSocket, marker management, clustering, side panel, themes,
 *          DC baseline markers, multi-alert aggregation.
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
  // id → { leafletMarker (null for DC-owned alerts), data }
  let markerMap    = {};
  let geoBoundsMap = {};  // id → L.Rectangle (hover overlay)
  let clusterGroup;       // L.markerClusterGroup
  let activeMarkerId = null;
  let activeDCName   = null;  // currently open DC panel
  let ws;
  let wsReconnectTimer;
  let effects = {};
  // Heatmap region definitions fetched from /api/config; read by heatmap.js effect.
  let heatmapRegions = [];

  // DC baseline markers: name → { leafletMarker, alerts: {alertId→data}, lat, lng }
  let dcMarkers = {};

  // SG NPC boundary overlay (L.geoJSON layer, lazy-loaded)
  let npcLayer     = null;
  let npcVisible   = false;
  let npcLoading   = false;

  // ── Initialise map ────────────────────────────────────────────────────────────

  // Singapore bounding box — matches geo/slice.go RegionBounds["SG"]
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

    clusterGroup = L.markerClusterGroup({ chunkedLoading: true, zoomToBoundsOnClick: false });
    clusterGroup.addTo(map);

    // Keyboard shortcut: Esc closes panel.
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') MapWatch.closePanel();
    });

    // Mouse coordinate display (bottom-left corner).
    const coordEl = document.getElementById('map-coords');
    if (coordEl) {
      map.on('mousemove', (e) => {
        coordEl.style.display = 'block';
        coordEl.textContent = e.latlng.lat.toFixed(4) + ', ' + e.latlng.lng.toFixed(4);
      });
      map.on('mouseout', () => { coordEl.style.display = 'none'; });
    }

    // Fetch runtime config (Prometheus external URL, locations, etc.) from server.
    fetchConfig();
    connectWS();
  }

  // ── Runtime config ────────────────────────────────────────────────────────────

  function fetchConfig() {
    fetch('/api/config')
      .then(r => r.json())
      .then(cfg => {
        if (cfg && cfg.locations && cfg.locations.length) initDCMarkers(cfg.locations);
        if (cfg && Array.isArray(cfg.heatmapRegions)) {
          heatmapRegions = cfg.heatmapRegions;
          window.MapWatch.heatmapRegions = heatmapRegions;
          // Markers may have already arrived via WS before config loaded.
          // Re-run effects so the heatmap can draw with the now-known regions.
          runEffects({ type: 'config.loaded' });
        }
      })
      .catch(() => { /* use defaults */ });
  }

  // ── DC baseline markers ───────────────────────────────────────────────────────
  //
  // Known infrastructure locations from config are shown as small green "healthy"
  // dots.  When alerts fire for a DC, its dot changes colour/size and shows a count
  // badge.  All alerts for one DC are aggregated into a single clickable marker.

  function initDCMarkers(locations) {
    for (const loc of locations) {
      const lm = L.marker([loc.lat, loc.lng], {
        icon: makeDCIcon(0, null),
        zIndexOffset: -200,   // keep below alert markers
      });
      lm.bindTooltip(makeDCTooltipHtml(loc.name, {}), {
        permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
      });
      lm.on('click', () => MapWatch.openDCPanel(loc.name));
      dcMarkers[loc.name] = { leafletMarker: lm, alerts: {}, lat: loc.lat, lng: loc.lng };
      lm.addTo(map);   // DC markers live directly on the map, not in cluster/spread
    }

    // Alerts may have arrived via WS before config loaded (dcMarkers was empty).
    // Re-aggregate any individual markers that belong to a known DC.
    for (const [id, entry] of Object.entries(markerMap)) {
      const dcName = getDCForAlert(entry.data);
      if (!dcName) continue;
      // Remove the stray individual Leaflet marker from the cluster layer.
      if (entry.leafletMarker) {
        clusterGroup.removeLayer(entry.leafletMarker);
        if (geoBoundsMap[id]) { map.removeLayer(geoBoundsMap[id]); delete geoBoundsMap[id]; }
        entry.leafletMarker = null;
      }
      dcMarkers[dcName].alerts[id] = entry.data;
      updateDCMarker(dcName);
    }
  }

  /**
   * Build a Leaflet DivIcon for a DC marker.
   * @param {number}      alertCount   number of active alerts (0 = healthy)
   * @param {string|null} worstSev     worst severity among active alerts
   */
  function makeDCIcon(alertCount, worstSev) {
    let color, pulse, dotSize;
    if (alertCount === 0) {
      color   = '#3fb950';  // green — healthy
      pulse   = 'mw-breathe';
      dotSize = 14;
    } else if (worstSev === 'critical') {
      color   = SEVERITY_COLORS.critical;
      pulse   = 'mw-pulse';
      dotSize = 20;
    } else if (worstSev === 'warning') {
      color   = SEVERITY_COLORS.warning;
      pulse   = '';
      dotSize = 18;
    } else {
      color   = SEVERITY_COLORS.info;
      pulse   = '';
      dotSize = 16;
    }

    const margin  = (22 - dotSize) / 2;
    const badge   = alertCount > 1
      ? `<span class="mw-dc-badge">${alertCount > 99 ? '99+' : alertCount}</span>`
      : '';

    return L.divIcon({
      className: '',
      html: `<div class="mw-dc-wrap">` +
              `<div class="mw-marker ${pulse}" ` +
                   `style="background:${color};border-color:${color};` +
                          `width:${dotSize}px;height:${dotSize}px;margin:${margin}px">` +
              `</div>${badge}</div>`,
      iconSize:      [28, 28],
      iconAnchor:    [14, 14],
      tooltipAnchor: [14, 0],
    });
  }

  /** Tooltip content for a DC marker — shows up to 3 alerts, then "+N more". */
  function makeDCTooltipHtml(name, alerts) {
    const list  = Object.values(alerts);
    const count = list.length;

    if (count === 0) {
      return `<div class="mw-tt">` +
               `<div class="mw-tt-title">${e(name)}</div>` +
               `<div class="mw-tt-sev" style="color:#3fb950">HEALTHY</div>` +
             `</div>`;
    }

    const worst   = worstSeverityOf(list);
    const preview = list.slice(0, 3).map(a => {
      const col = severityColor(a.severity || 'unknown');
      return `<div class="mw-tt-row" style="color:${col}">● ${e(a.alertname || a.id)}</div>`;
    }).join('');
    const more = count > 3
      ? `<div class="mw-tt-row" style="color:#8b949e">…and ${count - 3} more</div>`
      : '';

    return `<div class="mw-tt">` +
             `<div class="mw-tt-title">${e(name)}</div>` +
             `<div class="mw-tt-sev" style="color:${severityColor(worst)}">` +
               `${count} ALERT${count !== 1 ? 'S' : ''}` +
             `</div>` +
             preview + more +
             `<div class="mw-tt-hint">Click to view all ↗</div>` +
           `</div>`;
  }

  /** Return the worst severity label from an array of alert data objects. */
  function worstSeverityOf(alerts) {
    for (const a of alerts) if (a.severity === 'critical') return 'critical';
    for (const a of alerts) if (a.severity === 'warning')  return 'warning';
    return alerts.length ? (alerts[0].severity || 'unknown') : 'unknown';
  }

  /**
   * Find which DC (if any) owns this alert.
   * Matches by alert.labels.datacenter → dcMarkers key.
   */
  function getDCForAlert(data) {
    if (data.labels && data.labels.datacenter) {
      const name = data.labels.datacenter;
      if (dcMarkers[name]) return name;
    }
    return null;
  }

  /** Recompute and redraw the DC marker icon + tooltip after its alert set changes. */
  function updateDCMarker(dcName) {
    const dc = dcMarkers[dcName];
    if (!dc) return;
    const list  = Object.values(dc.alerts);
    const count = list.length;
    const worst = count > 0 ? worstSeverityOf(list) : null;

    dc.leafletMarker.setIcon(makeDCIcon(count, worst));
    dc.leafletMarker.unbindTooltip();
    dc.leafletMarker.bindTooltip(makeDCTooltipHtml(dcName, dc.alerts), {
      permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
    });

    // Live-refresh the panel if this DC's panel is currently open.
    if (activeDCName === dcName) renderDCPanel(dcName, dc.alerts);
  }

  // ── WebSocket ─────────────────────────────────────────────────────────────────

  function connectWS() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url   = `${proto}://${location.host}/ws`;
    console.log('[MapWatch] WS connecting to', url);
    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log('[MapWatch] WS connected');
      document.getElementById('ws-status').classList.add('connected');
      clearTimeout(wsReconnectTimer);
    };

    ws.onclose = (evt) => {
      console.warn('[MapWatch] WS closed code=' + evt.code + ' reason=' + (evt.reason || 'none') + ' — reconnecting in 3s');
      document.getElementById('ws-status').classList.remove('connected');
      wsReconnectTimer = setTimeout(connectWS, 3000);
    };

    ws.onerror = (err) => {
      console.error('[MapWatch] WS error', err);
      ws.close();
    };

    ws.onmessage = (evt) => {
      let msg;
      try { msg = JSON.parse(evt.data); } catch { return; }
      console.log('[MapWatch] WS event', msg.type, msg.marker ? 'id=' + msg.marker.id + ' sev=' + msg.marker.severity : 'id=' + msg.id);
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
    const sev      = d.severity || 'unknown';
    const color    = severityColor(sev);
    const instance = (d.labels && d.labels.instance) || '';
    const dc       = (d.labels && d.labels.datacenter) || '';
    const summary  = (d.annotations && d.annotations.summary) || '';

    let html = `<div class="mw-tt">
      <div class="mw-tt-title">${e(d.alertname || d.id)}</div>
      <div class="mw-tt-sev" style="color:${color}">${sev.toUpperCase()}</div>`;

    if (instance) html += `<div class="mw-tt-row">${e(instance)}</div>`;
    if (dc)       html += `<div class="mw-tt-row">DC: ${e(dc)}</div>`;
    if (summary)  html += `<div class="mw-tt-row">${e(summary)}</div>`;

    html += `<div class="mw-tt-hint">Click for details ↗</div></div>`;
    return html;
  }

  function upsertMarker(data, isNew) {
    console.log('[MapWatch] upsertMarker', isNew ? 'ADD' : 'UPDATE', 'id=' + data.id,
                'sev=' + data.severity, 'lat=' + data.lat, 'lng=' + data.lng);

    // ── DC-owned alert: aggregate into the DC baseline marker ─────────────────
    const dcName = getDCForAlert(data);
    if (dcName) {
      dcMarkers[dcName].alerts[data.id] = data;
      updateDCMarker(dcName);
      // Keep a markerMap entry (null leafletMarker) so openPanel / loadLinks work.
      if (markerMap[data.id]) {
        markerMap[data.id].data = data;
      } else {
        markerMap[data.id] = { leafletMarker: null, data };
      }
      return;
    }

    // ── Regular alert: individual Leaflet marker ───────────────────────────────
    if (markerMap[data.id]) {
      const { leafletMarker } = markerMap[data.id];
      if (leafletMarker) {
        const lat = effectiveLat(data);
        const lng = effectiveLng(data);
        leafletMarker.setLatLng([lat, lng]);
        leafletMarker.setIcon(makeIcon(data));
        leafletMarker.unbindTooltip();
        leafletMarker.bindTooltip(makeTooltipHtml(data), {
          permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
        });
      }
      markerMap[data.id].data = data;
    } else {
      const lat = effectiveLat(data);
      const lng = effectiveLng(data);
      const lm  = L.marker([lat, lng], { icon: makeIcon(data) });

      lm.bindTooltip(makeTooltipHtml(data), {
        permanent: false, direction: 'top', className: 'mw-tooltip', opacity: 1,
      });
      lm.on('click', () => MapWatch.openPanel(data.id));

      markerMap[data.id] = { leafletMarker: lm, data };
      addToActiveLayer(lm);
      console.log('[MapWatch] marker added to layer, pulse=' + (data.severity === 'critical'));
    }
  }

  function removeMarker(id) {
    if (!markerMap[id]) return;
    const { leafletMarker, data } = markerMap[id];

    // Remove from DC aggregation if this was a DC-owned alert.
    const dcName = getDCForAlert(data);
    if (dcName && dcMarkers[dcName]) {
      delete dcMarkers[dcName].alerts[id];
      updateDCMarker(dcName);
    }

    if (leafletMarker) {
      clusterGroup.removeLayer(leafletMarker);
    }
    delete markerMap[id];

    if (geoBoundsMap[id]) {
      map.removeLayer(geoBoundsMap[id]);
      delete geoBoundsMap[id];
    }
    if (activeMarkerId === id) MapWatch.closePanel();
  }

  function addToActiveLayer(lm) {
    clusterGroup.addLayer(lm);
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

  // ── Theme switcher ────────────────────────────────────────────────────────────

  function setTheme(name) {
    const t = THEMES[name];
    if (!t) return;
    tileLayer.setUrl(t.url);
    ['dark', 'light', 'satellite'].forEach((n) => {
      document.getElementById('btn-' + n).classList.toggle('active', n === name);
    });
  }

  // ── Side panel ────────────────────────────────────────────────────────────────

  /** Open the individual alert detail panel. */
  function openPanel(id) {
    const entry = markerMap[id];
    if (!entry) return;
    activeMarkerId = id;
    activeDCName   = null;
    renderPanel(entry.data);
    document.getElementById('panel').classList.add('open');
    loadLinks(id);
  }

  /**
   * Open the DC aggregated panel showing all alerts for a datacenter location.
   * Each alert row is clickable to drill into individual alert details.
   */
  function openDCPanel(dcName) {
    const dc = dcMarkers[dcName];
    if (!dc) return;
    activeDCName   = dcName;
    activeMarkerId = null;
    renderDCPanel(dcName, dc.alerts);
    document.getElementById('panel').classList.add('open');
  }

  function closePanel() {
    document.getElementById('panel').classList.remove('open');
    activeMarkerId = null;
    activeDCName   = null;
  }

  /**
   * Render the DC aggregated panel.
   * Shows a severity summary bar, severity chips, and a scrollable alert list.
   */
  function renderDCPanel(dcName, alerts) {
    const alertList = Object.values(alerts);
    document.getElementById('panel-title').textContent = dcName;
    const content = document.getElementById('panel-content');

    if (alertList.length === 0) {
      content.innerHTML =
        `<div style="text-align:center;padding:32px 0;color:#3fb950">` +
          `<div style="font-size:36px;margin-bottom:10px">✓</div>` +
          `<div style="font-weight:600;font-size:14px">All systems operational</div>` +
        `</div>`;
      return;
    }

    // Severity summary bar — proportional colour segments.
    const sevCounts = {};
    for (const a of alertList) {
      const s = a.severity || 'unknown';
      sevCounts[s] = (sevCounts[s] || 0) + 1;
    }
    const total = alertList.length;
    const SEV_ORDER = ['critical', 'warning', 'info', 'unknown'];
    const barSegs = SEV_ORDER
      .filter(s => sevCounts[s])
      .map(s => {
        const pct = ((sevCounts[s] / total) * 100).toFixed(1);
        return `<div class="dc-sev-seg" style="width:${pct}%;background:${severityColor(s)}" ` +
                    `title="${sevCounts[s]} ${s}"></div>`;
      }).join('');

    const chips = SEV_ORDER
      .filter(s => sevCounts[s])
      .map(s => {
        const cls = SEVERITY_COLORS[s] ? s : 'unknown';
        return `<span class="severity-badge sev-${cls}">${sevCounts[s]} ${s}</span>`;
      }).join(' ');

    // Sort alerts: critical first, then warning, info, unknown, then by startsAt desc.
    const sevRank = { critical: 0, warning: 1, info: 2, unknown: 3 };
    const sorted  = [...alertList].sort((a, b) => {
      const sr = (sevRank[a.severity] || 3) - (sevRank[b.severity] || 3);
      if (sr !== 0) return sr;
      return new Date(b.startsAt || 0) - new Date(a.startsAt || 0);
    });

    const rows = sorted.map(a => {
      const sev      = a.severity || 'unknown';
      const cls      = SEVERITY_COLORS[sev] ? sev : 'unknown';
      const inst     = (a.labels && a.labels.instance) || '';
      const dur      = a.startsAt ? timeSince(new Date(a.startsAt)) : '';
      const summary  = (a.annotations && a.annotations.summary) || '';
      return `<div class="dc-alert-item" onclick="MapWatch.openPanel('${e(a.id)}')">` +
               `<span class="severity-badge sev-${cls}">${sev}</span>` +
               `<div class="dc-alert-body">` +
                 `<div class="dc-alert-name">${e(a.alertname || a.id)}</div>` +
                 (inst    ? `<div class="dc-alert-meta">${e(inst)}${dur ? ' · ' + e(dur) : ''}</div>` : '') +
                 (summary ? `<div class="dc-alert-summary">${e(summary)}</div>` : '') +
               `</div>` +
               `<span class="dc-alert-arrow">↗</span>` +
             `</div>`;
    }).join('');

    content.innerHTML =
      `<div style="margin-bottom:14px">` +
        `<div class="dc-sev-bar">${barSegs}</div>` +
        `<div style="margin-top:8px;display:flex;gap:6px;flex-wrap:wrap">${chips}</div>` +
      `</div>` +
      `<div class="panel-section">` +
        `<h3>${total} Active Alert${total !== 1 ? 's' : ''}</h3>` +
        `<div id="dc-alert-list">${rows}</div>` +
      `</div>`;
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
    const loading   = document.getElementById('links-loading');
    const errEl     = document.getElementById('links-error');
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
    if (secs < 60)    return secs + 's';
    if (secs < 3600)  return Math.floor(secs / 60) + 'm';
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

  // ── SG NPC Boundary overlay ───────────────────────────────────────────────────

  function styleNPCFeature(_feature) {
    return {
      color:       '#4fc3f7',
      weight:      1.5,
      opacity:     0.8,
      fillColor:   '#4fc3f7',
      fillOpacity: 0.06,
    };
  }

  function onEachNPCFeature(feature, layer) {
    const p = feature.properties || {};
    const name = p.NPC_NAME || p.Name || p.name || '';
    const div  = p.DIVISION  || p.Division  || '';
    if (name) {
      layer.bindTooltip(
        `<div class="mw-tt-title">${e(name)}</div>` +
        (div ? `<div class="mw-tt-row">Division: ${e(div)}</div>` : ''),
        { sticky: true, className: 'mw-tooltip', opacity: 1 }
      );
    }
  }

  function toggleNPCZones(btn) {
    if (npcLoading) return;

    // If already loaded, just toggle visibility.
    if (npcLayer) {
      npcVisible = !npcVisible;
      if (npcVisible) {
        npcLayer.addTo(map);
        btn && btn.classList.add('active');
      } else {
        npcLayer.remove();
        btn && btn.classList.remove('active');
      }
      return;
    }

    // First call — fetch and build the layer.
    npcLoading = true;
    btn && (btn.textContent = 'Loading…');

    fetch('/api/geojson/sg-npc-boundary')
      .then(r => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then(geojson => {
        npcLayer = L.geoJSON(geojson, {
          style:          styleNPCFeature,
          onEachFeature:  onEachNPCFeature,
        }).addTo(map);
        npcVisible = true;
        npcLoading = false;
        btn && (btn.textContent = 'NPC Zones');
        btn && btn.classList.add('active');
      })
      .catch(err => {
        npcLoading = false;
        btn && (btn.textContent = 'NPC Zones');
        console.error('NPC zones load failed:', err);
        alert('Could not load NPC boundary data.\nRun: mapwatch download-sg');
      });
  }

  // ── Public API ────────────────────────────────────────────────────────────────

  window.MapWatch = {
    registerEffect,
    setTheme,
    openPanel,
    openDCPanel,
    closePanel,
    toggleNPCZones,
    // Heatmap region definitions — populated from /api/config by fetchConfig();
    // read by heatmap.js on every effect invocation.
    heatmapRegions: [],
    // Exposed for static export mode (pre-load markers without WS)
    loadStaticMarkers(markers) {
      for (const m of markers) upsertMarker(m, true);
    },
  };

  // Boot
  document.addEventListener('DOMContentLoaded', init);
})();
