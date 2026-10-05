DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM road_network_imports) THEN
        RAISE EXCEPTION 'Refusing to roll back road-network provenance while imports exist; preserve and review source manifests first';
    END IF;
END;
$$;

DROP INDEX roads_network_import_idx;
ALTER TABLE roads DROP COLUMN road_network_import_id;
DROP TABLE road_network_imports;
