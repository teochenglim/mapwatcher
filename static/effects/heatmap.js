/**
 * Effect: heatmap
 * Renders a Leaflet.heat density layer from all current markers.
 * Re-renders on every add/update/remove event.
 */
(function () {
  let heatLayer = null;

  MapWatch.registerEffect('heatmap', function (event, map, markerMap) {
    if (!['marker.add', 'marker.update', 'marker.remove'].includes(event.type)) return;

    const points = Object.values(markerMap).map(({ data }) => {
      const lat = data.lat + (data.offset ? data.offset.Lat : 0);
      const lng = data.lng + (data.offset ? data.offset.Lng : 0);
      const intensity = data.severity === 'critical' ? 1.0
                      : data.severity === 'warning'  ? 0.6
                      : 0.3;
      return [lat, lng, intensity];
    });

    if (heatLayer) {
      heatLayer.setLatLngs(points);
    } else {
      // L.heatLayer is provided by leaflet.heat plugin
      if (typeof L.heatLayer !== 'function') return;
      heatLayer = L.heatLayer(points, { radius: 25, blur: 15, maxZoom: 10 });
      // Heatmap is opt-in — not added to map by default.
      // Users can call: MapWatch.toggleHeatmap()
    }
  });

  let heatVisible = false;
  window.MapWatch.toggleHeatmap = function () {
    if (!heatLayer) return;
    heatVisible = !heatVisible;
    if (heatVisible) {
      heatLayer.addTo(map);
    } else {
      map.removeLayer(heatLayer);
    }
  };
})();
