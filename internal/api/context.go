package api

import (
	"context"

	"microservice/internal/shop"
)

type ctxKey string

const ctxKeyShop ctxKey = "shop"

func WithShop(ctx context.Context, s *shop.Shop) context.Context {
	return context.WithValue(ctx, ctxKeyShop, s)
}

func ShopFromContext(ctx context.Context) *shop.Shop {
	v := ctx.Value(ctxKeyShop)
	if v == nil {
		return nil
	}
	s, _ := v.(*shop.Shop)
	return s
}


