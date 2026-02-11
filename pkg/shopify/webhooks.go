package shopify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type webhookCreateRequest struct {
	Webhook webhookPayload `json:"webhook"`
}

type webhookPayload struct {
	Topic   string `json:"topic"`
	Address string `json:"address"`
	Format  string `json:"format"`
}

type webhookCreateResponse struct {
	Webhook struct {
		ID int64 `json:"id"`
	} `json:"webhook"`
}

func (c Client) CreateWebhook(ctx context.Context, topic string, address string) error {
	topic = strings.TrimSpace(topic)
	address = strings.TrimSpace(address)
	if topic == "" || address == "" {
		return fmt.Errorf("missing topic or address")
	}

	req := webhookCreateRequest{
		Webhook: webhookPayload{
			Topic:   topic,
			Address: address,
			Format:  "json",
		},
	}
	var resp webhookCreateResponse
	status, err := c.doJSON(ctx, http.MethodPost, "/webhooks.json", req, &resp)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("create webhook failed: status=%d", status)
	}
	return nil
}


