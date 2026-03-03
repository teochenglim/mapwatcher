# SG Roads Layer — Data Notes

## Source

**SLA National Map Line** (dataset `d_10480c0b59e65663dfae1028ff4aa8bb`)
https://data.gov.sg/datasets/d_10480c0b59e65663dfae1028ff4aa8bb/view
Published by the Singapore Land Authority (SLA). Updated regularly (last seen: Feb 2026).

## Raw File Structure

The downloaded GeoJSON (`SINGAPOREMAP_LINE`) bundles five feature types in a single file:

| FOLDERPATH                  | Count  | Description                      |
|-----------------------------|--------|----------------------------------|
| `Layers/Contour_250K`       | 7,723  | Elevation contour lines — **excluded** |
| `Layers/Major_Road`         | 7,310  | Named major roads                |
| `Layers/Expressway_Sliproad`| 1,129  | On/off ramps and slip roads      |
| `Layers/Expressway`         |   726  | Expressways (PIE, CTE, AYE …)   |
| `Layers/International_bdy`  |     1  | SG–MY border line — **excluded** |

## Filtering

`DownloadRoads` downloads the full ~605 MB file, then filters in-memory to keep
only the three road `FOLDERPATH` values, producing a **~3.4 MB** output file.

Kept layers:
- `Layers/Expressway`
- `Layers/Expressway_Sliproad`
- `Layers/Major_Road`

Result: **9,165 road features**, suitable for browser rendering.

## Feature Properties

Each feature is a `LineString` with properties:

| Property     | Example                    | Notes                        |
|--------------|----------------------------|------------------------------|
| `OBJECTID`   | `28678`                    | Internal SLA identifier      |
| `NAME`       | `"ADMIRALTY ROAD WEST"`    | Road name (may be empty)     |
| `FOLDERPATH` | `"Layers/Major_Road"`      | Road classification          |
| `SYMBOLID`   | `3`                        | SLA render symbol (ignore)   |
| `SHAPE.LEN`  | `31.45`                    | Segment length in metres     |

Coordinates are `[lng, lat, elevation]` — the elevation component (always `0`) is
stripped automatically by Leaflet's `L.geoJSON` when rendering.
