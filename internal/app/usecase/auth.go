package usecase

import (
	"context"
	"errors"
	"strings"

	"tg-tui/internal/app"
	"tg-tui/internal/domains"
	"tg-tui/internal/ports/outbound"
)

type Auth struct {
	client outbound.TelegramClient
}

func NewAuth(client outbound.TelegramClient) Auth {
	return Auth{client: client}
}

func (u Auth) BeginQRLogin(ctx context.Context) (app.AuthState, domains.AccountSession, error) {
	if u.client == nil {
		return app.AuthState{}, domains.AccountSession{}, errors.New("telegram client is not initialized")
	}

	nextState, err := u.client.BeginQRLogin(ctx)
	if err != nil {
		return nextState, domains.AccountSession{}, err
	}

	session, err := u.client.Session(ctx)
	return nextState, session, err
}

func (u Auth) CompleteQRLogin(ctx context.Context) (app.AuthState, domains.AccountSession, error) {
	if u.client == nil {
		return app.AuthState{}, domains.AccountSession{}, errors.New("telegram client is not initialized")
	}

	nextState, err := u.client.CompleteQRLogin(ctx)
	if err != nil {
		return nextState, domains.AccountSession{}, err
	}

	session, err := u.client.Session(ctx)
	return nextState, session, err
}

func (u Auth) ResendCode(ctx context.Context) (app.AuthState, domains.AccountSession, error) {
	if u.client == nil {
		return app.AuthState{}, domains.AccountSession{}, errors.New("telegram client is not initialized")
	}

	nextState, err := u.client.ResendCode(ctx)
	if err != nil {
		return nextState, domains.AccountSession{}, err
	}

	session, err := u.client.Session(ctx)
	return nextState, session, err
}

func (u Auth) SubmitInput(ctx context.Context, step app.AuthStep, value string) (app.AuthState, domains.AccountSession, error) {
	if u.client == nil {
		return app.AuthState{}, domains.AccountSession{}, errors.New("telegram client is not initialized")
	}

	trimmed := strings.TrimSpace(value)
	if step != app.AuthStepQR && trimmed == "" {
		return app.AuthState{}, domains.AccountSession{}, errors.New("auth input cannot be empty")
	}

	var (
		nextState app.AuthState
		err       error
	)

	switch step {
	case app.AuthStepQR:
		nextState, err = u.client.CompleteQRLogin(ctx)
	case app.AuthStepCode:
		nextState, err = u.client.SubmitCode(ctx, trimmed)
	case app.AuthStepPassword:
		nextState, err = u.client.SubmitPassword(ctx, trimmed)
	default:
		nextState, err = u.client.SubmitPhone(ctx, trimmed)
	}

	if err != nil {
		return nextState, domains.AccountSession{}, err
	}

	session, err := u.client.Session(ctx)
	return nextState, session, err
}
