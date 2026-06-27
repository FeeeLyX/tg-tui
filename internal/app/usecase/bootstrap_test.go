package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/FeeeLyX/tg-tui/internal/app"
	"github.com/FeeeLyX/tg-tui/internal/domains"
	"github.com/FeeeLyX/tg-tui/internal/ports/outbound"
)

type fakeCache struct {
	chats    []domains.ChatSummary
	messages map[domains.ChatID][]domains.Message
}

func (f *fakeCache) SaveChats(_ context.Context, _ []domains.ChatSummary) error { return nil }
func (f *fakeCache) LoadChats(_ context.Context) ([]domains.ChatSummary, error) { return f.chats, nil }
func (f *fakeCache) SaveMessages(_ context.Context, _ domains.ChatID, _ []domains.Message) error {
	return nil
}
func (f *fakeCache) LoadMessages(_ context.Context, chatID domains.ChatID) ([]domains.Message, error) {
	return f.messages[chatID], nil
}
func (f *fakeCache) Close() error { return nil }

type fakeTelegramClient struct {
	session   domains.AccountSession
	authState app.AuthState
	startErr  error
	started   bool
	closed    bool
}

func (f *fakeTelegramClient) Session(_ context.Context) (domains.AccountSession, error) {
	return f.session, nil
}
func (f *fakeTelegramClient) Start(_ context.Context) error {
	f.started = true
	return f.startErr
}
func (f *fakeTelegramClient) AuthState(_ context.Context) (app.AuthState, error) {
	return f.authState, nil
}
func (f *fakeTelegramClient) BeginQRLogin(_ context.Context) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) CompleteQRLogin(_ context.Context) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) SubmitPhone(_ context.Context, _ string) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) ResendCode(_ context.Context) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) SubmitCode(_ context.Context, _ string) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) SubmitPassword(_ context.Context, _ string) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeTelegramClient) ListPrivateChats(_ context.Context) ([]domains.ChatSummary, error) {
	return nil, nil
}
func (f *fakeTelegramClient) LoadMessages(_ context.Context, _ domains.ChatID, _ int) ([]domains.Message, error) {
	return nil, nil
}
func (f *fakeTelegramClient) SendMessage(_ context.Context, _ domains.ChatID, _ string, _ int64) (domains.Message, error) {
	return domains.Message{}, nil
}
func (f *fakeTelegramClient) Updates() <-chan domains.AppEvent { return nil }
func (f *fakeTelegramClient) Close() error {
	f.closed = true
	return nil
}

func TestBootstrapRun_MissingCredentials(t *testing.T) {
	b := Bootstrapper{}

	state, client := b.Run(context.Background(), app.Config{}, nil)
	if client != nil {
		t.Fatalf("expected no client when credentials are missing")
	}
	if state.Error == nil {
		t.Fatalf("expected credentials error")
	}
	if state.Status != "Telegram credentials required" {
		t.Fatalf("unexpected status: %s", state.Status)
	}
}

func TestBootstrapRun_LoadsCacheAndStartsClient(t *testing.T) {
	chatID := domains.ChatID(42)
	cache := &fakeCache{
		chats: []domains.ChatSummary{{ID: chatID, Title: "chat"}},
		messages: map[domains.ChatID][]domains.Message{
			chatID: {{ID: 1, ChatID: chatID, Text: "hello"}},
		},
	}

	fakeClient := &fakeTelegramClient{
		session:   domains.AccountSession{Authorized: true, UpdatedAt: time.Now()},
		authState: app.AuthState{Step: app.AuthStepCode},
	}

	b := Bootstrapper{
		NewTelegramClient: func(_ app.Config) outbound.TelegramClient {
			return fakeClient
		},
	}

	cfg := app.Config{TelegramAPIID: 1, TelegramAPIHash: "hash"}
	state, client := b.Run(context.Background(), cfg, cache)
	if client == nil {
		t.Fatalf("expected started client")
	}
	if !fakeClient.started {
		t.Fatalf("expected client to be started")
	}
	if state.ActiveChatID != chatID {
		t.Fatalf("expected active chat to be restored from cache")
	}
	if got := len(state.MessagesByChat[chatID]); got != 1 {
		t.Fatalf("expected cached messages restored, got %d", got)
	}
	if !state.Session.Authorized {
		t.Fatalf("expected authorized session in state")
	}
}
