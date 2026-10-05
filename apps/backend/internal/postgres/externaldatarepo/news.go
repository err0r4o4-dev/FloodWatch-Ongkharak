package externaldatarepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

func projectNewsArticle(ctx context.Context, tx pgx.Tx, code, recordID string, record externaldata.Record, retrievedAt time.Time) error {
	identity := uuid.NewSHA1(uuid.NameSpaceURL, []byte(code+":"+record.News.ArticleID)).String()
	_, err := tx.Exec(ctx, `INSERT INTO external_data_news_articles
        (source_code,article_id,id,current_record_id,published_at,first_retrieved_at,last_retrieved_at,updated_at,history_total)
        VALUES ($1,$2,$3,$4,$5,$6,$6,$6,(SELECT count(*) FROM external_data_records WHERE source_code=$1 AND record_key=$2 AND kind='NEWS_REPORT' AND provenance='MEDIA'))
        ON CONFLICT (source_code,article_id) DO UPDATE SET
            current_record_id=EXCLUDED.current_record_id,published_at=EXCLUDED.published_at,
            last_retrieved_at=GREATEST(external_data_news_articles.last_retrieved_at,EXCLUDED.last_retrieved_at),
            updated_at=CASE WHEN external_data_news_articles.current_record_id<>EXCLUDED.current_record_id THEN GREATEST(external_data_news_articles.updated_at,EXCLUDED.updated_at) ELSE external_data_news_articles.updated_at END,
            history_total=EXCLUDED.history_total`, code, record.News.ArticleID, identity, recordID, record.IssuedAt, retrievedAt)
	if err != nil {
		return fmt.Errorf("project news publication: %w", err)
	}
	return nil
}

func (store *Store) NewsEvidence(ctx context.Context, limit, offset int) (newsannouncement.EvidenceSnapshot, error) {
	return store.newsEvidence(ctx, "FM91_NEWS", limit, offset)
}

func (store *Store) newsEvidence(ctx context.Context, code string, limit, offset int) (newsannouncement.EvidenceSnapshot, error) {
	if !validSourceCode(code) || limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return newsannouncement.EvidenceSnapshot{}, errInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return newsannouncement.EvidenceSnapshot{}, fmt.Errorf("begin news evidence: %w", err)
	}
	defer tx.Rollback(ctx)
	result := newsannouncement.EvidenceSnapshot{Records: []newsannouncement.Evidence{}}
	if err := tx.QueryRow(ctx, `SELECT enabled,last_success_at,last_error_code FROM external_data_sources WHERE code=$1`, code).Scan(&result.Enabled, &result.LastSuccessAt, &result.ErrorCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, externaldata.ErrNotFound
		}
		return result, fmt.Errorf("read news source health: %w", err)
	}
	if result.Enabled {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM external_data_news_articles WHERE source_code=$1`, code).Scan(&result.Total); err != nil {
			return result, err
		}
		rows, err := tx.Query(ctx, `SELECT news.id::text,record.payload,news.first_retrieved_at,news.last_retrieved_at,news.updated_at,news.history_total
            FROM external_data_news_articles news JOIN external_data_records record ON record.id=news.current_record_id
            WHERE news.source_code=$1 AND record.source_code=news.source_code AND record.record_key=news.article_id AND record.kind='NEWS_REPORT' AND record.provenance='MEDIA'
            ORDER BY COALESCE(news.published_at,news.updated_at) DESC,news.article_id ASC LIMIT $2 OFFSET $3`, code, limit, offset)
		if err != nil {
			return result, fmt.Errorf("read published news page: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var entry newsannouncement.Evidence
			var payload []byte
			if err := rows.Scan(&entry.ID, &payload, &entry.RetrievedAt, &entry.LastRetrievedAt, &entry.UpdatedAt, &entry.HistoryTotal); err != nil {
				return result, err
			}
			if err := json.Unmarshal(payload, &entry.Record); err != nil {
				return result, err
			}
			result.Records = append(result.Records, entry)
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
