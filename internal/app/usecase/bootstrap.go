package usecase

import (
	"context"
	"fmt"

	"github.com/FeeeLyX/tg-tui/internal/app"
	"github.com/FeeeLyX/tg-tui/internal/ports/outbound"
)

type Logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

type Bootstrapper struct {
	NewTelegramClient func(config app.Config) outbound.TelegramClient
	Logger            Logger
}

func (b Bootstrapper) Run(ctx context.Context, config app.Config, cache outbound.ChatCache) (app.State, outbound.TelegramClient) {
	state := b.newInitialState(config)
	b.hydrateFromCache(ctx, cache, &state)

	if err := config.ValidateCredentials(); err != nil {
		b.errorf("credential validation failed: %v", err)
		state.Error = err
		state.Status = "Telegram credentials required"
		return state, nil
	}

	if b.NewTelegramClient == nil {
		err := fmt.Errorf("telegram client factory is not configured")
		b.errorf("%v", err)
		state.Error = err
		state.Status = "Telegram startup failed"
		return state, nil
	}

	tgClient, err := b.startClient(ctx, config)
	if err != nil {
		state.Error = err
		state.Status = "Telegram startup failed"
		return state, nil
	}

	b.syncClientSessionState(ctx, tgClient, &state)
	return state, tgClient
}

func (b Bootstrapper) newInitialState(config app.Config) app.State {
	state := app.NewState()
	state.Status = "Session bootstrap pending"
	state.CredentialSummary = config.CredentialSummary()
	state.CredentialNotice = config.CredentialNotice()
	return state
}

func (b Bootstrapper) hydrateFromCache(ctx context.Context, cache outbound.ChatCache, state *app.State) {
	if cache == nil {
		return
	}

	chats, err := cache.LoadChats(ctx)
	if err != nil || len(chats) == 0 {
		return
	}

	*state = app.ApplyCachedChats(*state, chats)
	if state.ActiveChatID != 0 {
		messages, loadErr := cache.LoadMessages(ctx, state.ActiveChatID)
		if loadErr == nil {
			*state = app.ApplyCachedMessages(*state, state.ActiveChatID, messages)
		}
	}
	state.Status = "Loaded cached data"
}

func (b Bootstrapper) startClient(ctx context.Context, config app.Config) (outbound.TelegramClient, error) {
	tgClient := b.NewTelegramClient(config)
	b.infof("telegram client start begin")
	if err := tgClient.Start(ctx); err != nil {
		b.errorf("telegram client start failed: %v", err)
		_ = tgClient.Close()
		return nil, fmt.Errorf("telegram startup failed: %w", err)
	}
	b.infof("telegram client started")
	return tgClient, nil
}

func (b Bootstrapper) syncClientSessionState(ctx context.Context, client outbound.TelegramClient, state *app.State) {
	session, err := client.Session(ctx)
	if err != nil {
		state.Error = err
		state.Status = "Telegram session bootstrap failed"
	} else {
		state.Session = session
	}

	authState, err := client.AuthState(ctx)
	if err == nil {
		state.AuthState = authState
	}
	b.infof("telegram auth state step: %s", state.AuthState.Step)

	if state.Session.Authorized {
		state.Status = "Authorized. Chat sync is next."
	} else {
		state.Status = "Awaiting Telegram login"
	}
}

func (b Bootstrapper) infof(format string, args ...any) {
	if b.Logger != nil {
		b.Logger.Infof(format, args...)
	}
}

func (b Bootstrapper) errorf(format string, args ...any) {
	if b.Logger != nil {
		b.Logger.Errorf(format, args...)
	}
}
