# Map boundary (future Phase 5)

React-Leaflet/Leaflet are dependencies for later map work, not a Phase 0 map. Future map modes are `SITUATION` and `ROUTE`, with separately selectable flood, water, risk, road, CCTV, community, safe-area, and administrative layers. Define GeoJSON and coordinate reference expectations at the API boundary before adding spatial rendering; PostGIS geometry and indexes follow actual query needs.

The map must distinguish unknown and stale road states from passable routes. Text labels, a legend, keyboard alternatives, touch-friendly controls, and source/time disclosure are required alongside color. A future routing mode must never suggest a route is safe merely because no hazard data was returned.
