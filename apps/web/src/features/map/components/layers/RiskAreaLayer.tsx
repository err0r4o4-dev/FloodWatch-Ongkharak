import { GeoJSON } from "react-leaflet";
import type { MapSelectHandler, RiskFeature } from "../../types";
import { effectiveRiskLevel } from "../../utils";

// Risk level color mapping - matching our badge design
const RISK_COLORS = {
  VERY_HIGH: {
    color: "#dc2626",      // Red
    fillColor: "#fca5a5",  // Light red
    fillOpacity: 0.35
  },
  HIGH: {
    color: "#ea580c",      // Orange
    fillColor: "#fdba74",  // Light orange
    fillOpacity: 0.32
  },
  MODERATE: {
    color: "#f59e0b",      // Amber/Yellow
    fillColor: "#fde047",  // Light yellow
    fillOpacity: 0.28
  },
  LOW: {
    color: "#059669",      // Green
    fillColor: "#6ee7b7",  // Light green
    fillOpacity: 0.25
  },
  UNKNOWN: {
    color: "#64748b",      // Gray
    fillColor: "#cbd5e1",  // Light gray
    fillOpacity: 0.2
  }
} as const;

export function RiskAreaLayer({ items, onSelect }: { items: RiskFeature[]; onSelect: MapSelectHandler }) {
  return <>{items.map((feature) => {
    const level = effectiveRiskLevel(feature);
    const colors = RISK_COLORS[level as keyof typeof RISK_COLORS] || RISK_COLORS.UNKNOWN;
    
    return <GeoJSON 
      key={feature.id} 
      data={feature.geometry as Parameters<typeof GeoJSON>[0]["data"]} 
      style={{ 
        color: colors.color, 
        fillColor: colors.fillColor, 
        fillOpacity: colors.fillOpacity, 
        weight: 2.5,
        opacity: 0.8
      }} 
      eventHandlers={{ 
        click: (event) => onSelect({ layer: "risk", feature }, [event.latlng.lng, event.latlng.lat]) 
      }} 
    />;
  })}</>;
}
