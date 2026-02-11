package adminaction

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func Insert(ctx context.Context, tx pgx.Tx, serviceID string, actionType ActionType, reason, actor string, metadata any) error {
	var s *string
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		str := string(b)
		s = &str
	}
	const q = `
INSERT INTO admin_actions (service_id, action_type, reason, actor, metadata)
VALUES ($1, $2, $3, $4, CAST($5 AS jsonb))
`
	_, err := tx.Exec(ctx, q, serviceID, string(actionType), reason, actor, s)
	return err
}


