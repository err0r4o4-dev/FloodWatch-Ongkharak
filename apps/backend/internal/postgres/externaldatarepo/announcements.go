package externaldatarepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/floodwatch-ongkharak/backend/internal/announcement"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/jackc/pgx/v5"
)

func (store *Store) Evidence(ctx context.Context) (announcement.EvidenceSnapshot, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return announcement.EvidenceSnapshot{}, fmt.Errorf("begin announcement evidence: %w", err)
	}
	defer tx.Rollback(ctx)
	result := announcement.EvidenceSnapshot{Records: []announcement.Evidence{}}
	if err := tx.QueryRow(ctx, `SELECT enabled,last_success_at,last_error_code FROM external_data_sources WHERE code='TMD_CAP'`).Scan(&result.Enabled, &result.LastSuccessAt, &result.ErrorCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, externaldata.ErrNotFound
		}
		return result, fmt.Errorf("read announcement health: %w", err)
	}
	if result.Enabled {
		rows, err := tx.Query(ctx, `SELECT r.id::text,r.payload,COALESCE(seen.first_seen,r.created_at),COALESCE(seen.last_seen,r.created_at) FROM external_data_records r LEFT JOIN LATERAL (SELECT min(b.retrieved_at) AS first_seen,max(b.retrieved_at) AS last_seen FROM external_data_batch_records br JOIN external_data_batches b ON b.id=br.batch_id WHERE br.record_id=r.id) seen ON true WHERE r.source_code='TMD_CAP' AND r.kind='OFFICIAL_ALERT' AND r.provenance='OFFICIAL' ORDER BY r.issued_at,r.created_at,r.id LIMIT 10001`)
		if err != nil {
			return result, fmt.Errorf("read announcement history: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var entry announcement.Evidence
			var payload []byte
			if err := rows.Scan(&entry.ID, &payload, &entry.RetrievedAt, &entry.LastRetrievedAt); err != nil {
				return result, err
			}
			if err := json.Unmarshal(payload, &entry.Record); err != nil {
				return result, err
			}
			result.Records = append(result.Records, entry)
			if len(result.Records) > 10000 {
				return result, errors.New("announcement history exceeds read bound")
			}
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
		rows.Close()
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
