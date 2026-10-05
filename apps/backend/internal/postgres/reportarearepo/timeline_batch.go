package reportarearepo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/jackc/pgx/v5"
)

type timelineRecord struct {
	AreaID string `json:"areaId"`
	reportarea.TimelineWindow
}

func persistTimelines(ctx context.Context, transaction pgx.Tx, plan reportarea.RefreshPlan) error {
	const batchSize = 2000
	records := make([]timelineRecord, 0, batchSize)
	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		payload, err := json.Marshal(records)
		if err != nil {
			return fmt.Errorf("encode report area timelines: %w", err)
		}
		_, err = transaction.Exec(ctx, `INSERT INTO report_area_timeline
			(area_id,window_start,window_end,computed_at,depth_band,previous_depth_band,trend,reported_trend,freshness,summary_reason,latest_observed_at,report_count,sample_count,evidence_ids)
			SELECT entry."areaId",entry."windowStart",entry."windowEnd",entry."computedAt",entry."depthBand",entry."previousDepthBand",
			entry.trend,entry."reportedTrend",entry.freshness,entry."summaryReason",entry."latestObservedAt",entry."reportCount",entry."sampleCount",
			ARRAY(SELECT value::uuid FROM jsonb_array_elements_text(entry."reportIds"))
			FROM jsonb_to_recordset($1::jsonb) AS entry("areaId" uuid,"windowStart" timestamptz,"windowEnd" timestamptz,"computedAt" timestamptz,
			"depthBand" text,"previousDepthBand" text,trend text,"reportedTrend" text,freshness text,"summaryReason" text,
			"latestObservedAt" timestamptz,"reportCount" integer,"sampleCount" integer,"reportIds" jsonb)`, string(payload))
		if err != nil {
			return fmt.Errorf("persist report area timeline batch: %w", err)
		}
		records = records[:0]
		return nil
	}
	for _, area := range plan.Areas {
		for _, window := range area.Timeline {
			records = append(records, timelineRecord{AreaID: area.Area.ID, TimelineWindow: window})
			if len(records) == batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	return flush()
}
