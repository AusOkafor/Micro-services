package events

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID         string `json:"id"`
	ServiceID  string `json:"serviceId"`
	EventType  string `json:"eventType"`
	Summary    string `json:"summary"`
	Actor      string `json:"actor"`
	OccurredAt string `json:"occurredAt"`
	Data       any    `json:"data,omitempty"`
}

func ListByService(ctx context.Context, db *pgxpool.Pool, serviceID string) ([]Event, error) {
	const q = `
SELECT id, service_id, event_type, summary, actor, occurred_at::text, COALESCE(data, '{}'::jsonb)
FROM service_events
WHERE service_id = $1
ORDER BY occurred_at ASC, created_at ASC
`
	rows, err := db.Query(ctx, q, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.ServiceID, &e.EventType, &e.Summary, &e.Actor, &e.OccurredAt, &e.Data); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}


