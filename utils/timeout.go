package utils

import (
	"context"
	"time"
)

func TimeoutCtx(duration time.Duration) context.Context {
	ctx := context.Background()
	if duration > 0 {
		ctx, _ = context.WithTimeout(context.Background(), duration)
	}
	return ctx
}
