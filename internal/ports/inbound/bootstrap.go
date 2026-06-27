package inbound

import (
	"context"

	"tg-tui/internal/app"
	"tg-tui/internal/ports/outbound"
)

type Bootstrapper interface {
	Run(ctx context.Context, config app.Config, cache outbound.ChatCache) (app.State, outbound.TelegramClient)
}
