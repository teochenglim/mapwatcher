/**
 * Effect: blink-critical
 * The pulse animation is already baked into the DivIcon HTML via the .mw-pulse CSS class.
 * This effect adds the class to updated markers in case severity changes to critical.
 */
MapWatch.registerEffect('blink-critical', function (event, map, markerMap) {
  if (event.type !== 'marker.add' && event.type !== 'marker.update') return;
  const m = event.marker;
  if (!m) return;

  const entry = markerMap[m.id];
  if (!entry) return;

  // DivIcon markers: find the inner div and toggle the pulse class.
  const iconEl = entry.leafletMarker.getElement && entry.leafletMarker.getElement();
  if (!iconEl) return;
  const dot = iconEl.querySelector('.mw-marker');
  if (!dot) return;

  const isPulsing = m.severity === 'critical' || (m.labels && m.labels.priority === 'P1');
  dot.classList.toggle('mw-pulse', isPulsing);
});
