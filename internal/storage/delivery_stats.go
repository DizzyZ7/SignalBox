package storage

import (
	"context"

	"github.com/DizzyZ7/SignalBox/internal/domain"
)

func (r *Repository) DeliveryStats(ctx context.Context) (domain.DeliveryStats, error) {
	var out domain.DeliveryStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending')::BIGINT,
			COUNT(*) FILTER (WHERE status = 'processing')::BIGINT,
			COUNT(*) FILTER (WHERE status = 'sent')::BIGINT,
			COUNT(*) FILTER (WHERE status = 'failed')::BIGINT
		FROM delivery_jobs
	`).Scan(&out.Pending, &out.Processing, &out.Sent, &out.Failed)
	if err != nil {
		return domain.DeliveryStats{}, err
	}
	return out, nil
}
