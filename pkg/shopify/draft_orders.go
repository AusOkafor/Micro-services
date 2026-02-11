package shopify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type DraftOrderCreateRequest struct {
	DraftOrder DraftOrder `json:"draft_order"`
}

type DraftOrder struct {
	LineItems []DraftOrderLineItem `json:"line_items"`
	Note      string              `json:"note,omitempty"`
	Currency  string              `json:"currency,omitempty"`
}

type DraftOrderLineItem struct {
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
	Price    string `json:"price"`
}

type DraftOrderCreateResponse struct {
	DraftOrder DraftOrderResult `json:"draft_order"`
}

type DraftOrderResult struct {
	ID         int64  `json:"id"`
	InvoiceURL string `json:"invoice_url"`
}

func (c Client) CreateDraftOrder(ctx context.Context, title string, amount string, currency string, note string) (draftOrderID string, checkoutURL string, err error) {
	// Use GraphQL instead of REST /draft_orders.json.
	// Some shops/apps are blocked from certain REST endpoints (protected customer data policy),
	// while GraphQL draftOrderCreate remains available when properly scoped.
	const mutation = `
mutation DraftOrderCreate($input: DraftOrderInput!) {
  draftOrderCreate(input: $input) {
    draftOrder {
      id
      invoiceUrl
    }
    userErrors {
      field
      message
    }
  }
}
`

	type gqlResp struct {
		Data struct {
			DraftOrderCreate struct {
				DraftOrder *struct {
					ID        string `json:"id"`
					InvoiceURL string `json:"invoiceUrl"`
				} `json:"draftOrder"`
				UserErrors []struct {
					Field   []string `json:"field"`
					Message string   `json:"message"`
				} `json:"userErrors"`
			} `json:"draftOrderCreate"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	// GraphQL input for a custom line item draft order.
	// Note: Shopify accepts decimals as strings for originalUnitPrice.
	input := map[string]any{
		"note": note,
		"lineItems": []map[string]any{
			{
				"title":             title,
				"quantity":          1,
				"originalUnitPrice": amount,
			},
		},
	}

	var resp gqlResp
	_, err = c.doJSON(ctx, http.MethodPost, "/graphql.json", map[string]any{
		"query":     mutation,
		"variables": map[string]any{"input": input},
	}, &resp)
	if err != nil {
		return "", "", err
	}
	if len(resp.Errors) > 0 {
		return "", "", fmt.Errorf("draftOrderCreate graphql error: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.DraftOrderCreate.UserErrors) > 0 {
		return "", "", fmt.Errorf("draftOrderCreate user error: %s", resp.Data.DraftOrderCreate.UserErrors[0].Message)
	}
	if resp.Data.DraftOrderCreate.DraftOrder == nil || resp.Data.DraftOrderCreate.DraftOrder.ID == "" {
		return "", "", fmt.Errorf("draftOrderCreate returned empty id")
	}

	// Convert GID -> numeric id string (we store numeric IDs elsewhere).
	gid := resp.Data.DraftOrderCreate.DraftOrder.ID
	last := gid
	if i := strings.LastIndex(gid, "/"); i >= 0 && i < len(gid)-1 {
		last = gid[i+1:]
	}
	last = strings.TrimSpace(last)
	if last == "" {
		last = gid
	}

	return last, resp.Data.DraftOrderCreate.DraftOrder.InvoiceURL, nil
}


