# Map boundary (Phase 2 spatial API, future Phase 5 UI)

React-Leaflet/Leaflet are dependencies for later map work, not a Phase 2 map UI. Future map modes are `SITUATION` and `ROUTE`, with separately selectable layers. Phase 2 spatial GET routes accept an optional `bbox=minLng,minLat,maxLng,maxLat`, return GeoJSON geometry in WGS84, and always paginate. Point, multipolygon, and multiline geometries are indexed by PostGIS GiST. A composite `/map/features` route is deferred until the UI has a concrete demand; do not fetch every page to simulate a global map.

The map must distinguish unknown and stale road states from passable routes. Text labels, a legend, keyboard alternatives, touch-friendly controls, and source/time disclosure are required alongside color. A future routing mode must never suggest a route is safe merely because no hazard data was returned.
