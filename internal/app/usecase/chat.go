package usecase

import (
	"context"
	"errors"
	"strings"

	"tg-tui/internal/domains"
	"tg-tui/internal/ports/outbound"
)

const (
	DefaultMessageLimit = 80
	MaxMessageLimit     = 200
	LoadMoreStep        = 80
)

type Chat struct {
	client outbound.TelegramClient
}

func NewChat(client outbound.TelegramClient) Chat {
	return Chat{client: client}
}

func (u Chat) ListPrivateChats(ctx context.Context) ([]domains.ChatSummary, error) {
	if u.client == nil {
		return nil, errors.New("telegram client is not initialized")
	}
	return u.client.ListPrivateChats(ctx)
}

func (u Chat) LoadMessages(ctx context.Context, chatID domains.ChatID, limit int) ([]domains.Message, int, error) {
	if u.client == nil {
		return nil, 0, errors.New("telegram client is not initialized")
	}
	normalizedLimit := normalizeMessageLimit(limit)
	messages, err := u.client.LoadMessages(ctx, chatID, normalizedLimit)
	return messages, normalizedLimit, err
}

func (u Chat) LoadMoreMessages(ctx context.Context, chatID domains.ChatID, currentLimit int) ([]domains.Message, int, error) {
	nextLimit := currentLimit + LoadMoreStep
	if nextLimit < DefaultMessageLimit {
		nextLimit = DefaultMessageLimit
	}
	if nextLimit > MaxMessageLimit {
		nextLimit = MaxMessageLimit
	}
	return u.LoadMessages(ctx, chatID, nextLimit)
}

func (u Chat) SendMessage(ctx context.Context, chatID domains.ChatID, text string, replyToMessageID int64) (domains.Message, error) {
	if u.client == nil {
		return domains.Message{}, errors.New("telegram client is not initialized")
	}
	body := strings.TrimSpace(text)
	if body == "" {
		return domains.Message{}, errors.New("message text cannot be empty")
	}
	return u.client.SendMessage(ctx, chatID, body, replyToMessageID)
}

func normalizeMessageLimit(limit int) int {
	if limit <= 0 {
		return DefaultMessageLimit
	}
	if limit > MaxMessageLimit {
		return MaxMessageLimit
	}
	return limit
}
