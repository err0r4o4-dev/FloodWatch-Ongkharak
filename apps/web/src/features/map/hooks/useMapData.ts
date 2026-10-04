import { useEffect, useMemo, useRef, useState } from "react";
import { fetchMapLayer } from "../api";
import { layerKeys } from "../types";
import type { LayerItems, LayerKey, LayerVisibility, MapBounds } from "../types";

type MapData = {
  items: LayerItems;
  failed: LayerKey[];
  truncated: LayerKey[];
  loading: boolean;
  previousLayers: LayerKey[];
};

const emptyItems = (): LayerItems => ({ flood: [], water: [], risk: [], roads: [], reports: [] });

export function useMapData(bounds: MapBounds | null, visible: LayerVisibility, retry: number): MapData {
  const [data, setData] = useState<MapData>({ items: emptyItems(), failed: [], truncated: [], loading: true, previousLayers: [] });
  const requestVersion = useRef(0);

  // Memoize bounds key to prevent unnecessary re-renders
  const boundsKey = useMemo(() => {
    if (!bounds) return null;
    return `${bounds.west},${bounds.south},${bounds.east},${bounds.north}`;
  }, [bounds?.west, bounds?.south, bounds?.east, bounds?.north]);

  // Memoize visibility key to prevent unnecessary re-renders
  const visibilityKey = useMemo(() => {
    return layerKeys.filter((key) => visible[key]).join(',');
  }, [visible.flood, visible.water, visible.risk, visible.roads, visible.reports]);

  useEffect(() => {
    const version = requestVersion.current + 1;
    requestVersion.current = version;
    if (!bounds) return;
    const controller = new AbortController();
    const enabled = layerKeys.filter((key) => visible[key]);
    setData((previous) => {
      const items = emptyItems();
      enabled.forEach((key) => Object.assign(items, { [key]: previous.items[key] }));
      return {
        items,
        failed: previous.failed.filter((key) => enabled.includes(key)),
        truncated: previous.truncated.filter((key) => enabled.includes(key)),
        loading: enabled.length > 0,
        previousLayers: enabled.filter((key) => previous.items[key].length > 0)
      };
    });
    if (enabled.length === 0) return () => controller.abort();

    let pending = enabled.length;
    const isCurrent = () => !controller.signal.aborted && requestVersion.current === version;
    const settleLayer = (key: LayerKey, result: { items: LayerItems[LayerKey]; hasMore: boolean } | null) => {
      if (!isCurrent()) return;
      pending -= 1;
      setData((previous) => {
        if (!isCurrent()) return previous;
        const items = { ...previous.items, [key]: result?.items ?? [] };
        const failed = result ? previous.failed.filter((failedKey) => failedKey !== key) : [...previous.failed, key];
        const truncated = result?.hasMore ? [...previous.truncated.filter((truncatedKey) => truncatedKey !== key), key] : previous.truncated.filter((truncatedKey) => truncatedKey !== key);
        return {
          items,
          failed: [...new Set(failed)],
          truncated,
          loading: pending > 0,
          previousLayers: previous.previousLayers.filter((previousKey) => previousKey !== key)
        };
      });
    };

    const timer = window.setTimeout(() => {
      enabled.forEach((key) => {
        void fetchMapLayer(key, bounds, controller.signal)
          .then((result) => settleLayer(key, result))
          .catch(() => settleLayer(key, null));
      });
    }, 250);

    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [boundsKey, visibilityKey, retry]);

  return data;
}
