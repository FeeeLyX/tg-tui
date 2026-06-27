package app

import (
	"context"

	"tg-tui/internal/domains"
)

type AuthStep string

const (
	AuthStepPhone    AuthStep = "phone"
	AuthStepCode     AuthStep = "code"
	AuthStepQR       AuthStep = "qr"
	AuthStepPassword AuthStep = "password"
)

type AuthState struct {
	Step    AuthStep
	Hint    string
	Phone   string
	Pending bool
}

type ChatCache interface {
	SaveChats(ctx context.Context, chats []domains.ChatSummary) error
	LoadChats(ctx context.Context) ([]domains.ChatSummary, error)
	SaveMessages(ctx context.Context, chatID domains.ChatID, messages []domains.Message) error
	LoadMessages(ctx context.Context, chatID domains.ChatID) ([]domains.Message, error)
	Close() error
}

type TelegramClient interface {
	Session(ctx context.Context) (domains.AccountSession, error)
	Start(ctx context.Context) error
	AuthState(ctx context.Context) (AuthState, error)
	BeginQRLogin(ctx context.Context) (AuthState, error)
	CompleteQRLogin(ctx context.Context) (AuthState, error)
	SubmitPhone(ctx context.Context, phone string) (AuthState, error)
	ResendCode(ctx context.Context) (AuthState, error)
	SubmitCode(ctx context.Context, code string) (AuthState, error)
	SubmitPassword(ctx context.Context, password string) (AuthState, error)
	ListPrivateChats(ctx context.Context) ([]domains.ChatSummary, error)
	LoadMessages(ctx context.Context, chatID domains.ChatID, limit int) ([]domains.Message, error)
	SendMessage(ctx context.Context, chatID domains.ChatID, text string) (domains.Message, error)
	Updates() <-chan domains.AppEvent
	Close() error
}
