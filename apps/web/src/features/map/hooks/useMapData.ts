import { useEffect, useState } from "react";
import { fetchMapLayer } from "../api";
import { layerKeys } from "../types";
import type { LayerItems, LayerKey, LayerVisibility, MapBounds } from "../types";

type MapData = {
  items: LayerItems;
  failed: LayerKey[];
  truncated: LayerKey[];
  loading: boolean;
};

const emptyItems = (): LayerItems => ({ flood: [], water: [], risk: [], roads: [] });

export function useMapData(bounds: MapBounds | null, visible: LayerVisibility, retry: number): MapData {
  const [data, setData] = useState<MapData>({ items: emptyItems(), failed: [], truncated: [], loading: true });

  useEffect(() => {
    if (!bounds) return;
    const controller = new AbortController();
    const enabled = layerKeys.filter((key) => visible[key]);
    setData({ items: emptyItems(), failed: [], truncated: [], loading: enabled.length > 0 });
    if (enabled.length === 0) return () => controller.abort();

    const timer = window.setTimeout(() => {
      void Promise.allSettled(enabled.map((key) => fetchMapLayer(key, bounds, controller.signal))).then((results) => {
        if (controller.signal.aborted) return;
        const items = emptyItems();
        const failed: LayerKey[] = [];
        const truncated: LayerKey[] = [];
        results.forEach((result, index) => {
          const key = enabled[index];
          if (result.status === "rejected") failed.push(key);
          else {
            if (key === "flood") items.flood = result.value.items as LayerItems["flood"];
            if (key === "water") items.water = result.value.items as LayerItems["water"];
            if (key === "risk") items.risk = result.value.items as LayerItems["risk"];
            if (key === "roads") items.roads = result.value.items as LayerItems["roads"];
            if (result.value.hasMore) truncated.push(key);
          }
        });
        setData({ items, failed, truncated, loading: false });
      });
    }, 250);

    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [bounds?.west, bounds?.south, bounds?.east, bounds?.north, visible.flood, visible.water, visible.risk, visible.roads, retry]);

  return data;
}
