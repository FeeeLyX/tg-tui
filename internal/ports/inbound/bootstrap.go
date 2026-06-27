package inbound

import (
	"context"

	"github.com/FeeeLyX/tg-tui/internal/app"
	"github.com/FeeeLyX/tg-tui/internal/ports/outbound"
)

type Bootstrapper interface {
	Run(ctx context.Context, config app.Config, cache outbound.ChatCache) (app.State, outbound.TelegramClient)
}
