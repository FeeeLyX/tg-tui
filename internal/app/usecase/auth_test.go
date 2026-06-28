package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/FeeeLyX/tg-tui/internal/app"
	"github.com/FeeeLyX/tg-tui/internal/domains"
)

type fakeAuthClient struct {
	session             domains.AccountSession
	nextState           app.AuthState
	submitPhoneInput    string
	submitCodeInput     string
	submitPasswordInput string
	resendCalled        bool
	beginQRCalled       bool
	completeQRCalled    bool
	sessionErr          error
	submitErr           error
}

func (f *fakeAuthClient) Session(_ context.Context) (domains.AccountSession, error) {
	if f.sessionErr != nil {
		return domains.AccountSession{}, f.sessionErr
	}
	return f.session, nil
}
func (f *fakeAuthClient) Start(_ context.Context) error { return nil }
func (f *fakeAuthClient) AuthState(_ context.Context) (app.AuthState, error) {
	return app.AuthState{}, nil
}
func (f *fakeAuthClient) BeginQRLogin(_ context.Context) (app.AuthState, error) {
	f.beginQRCalled = true
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) CompleteQRLogin(_ context.Context) (app.AuthState, error) {
	f.completeQRCalled = true
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) SubmitPhone(_ context.Context, phone string) (app.AuthState, error) {
	f.submitPhoneInput = phone
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) ResendCode(_ context.Context) (app.AuthState, error) {
	f.resendCalled = true
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) SubmitCode(_ context.Context, code string) (app.AuthState, error) {
	f.submitCodeInput = code
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) SubmitPassword(_ context.Context, password string) (app.AuthState, error) {
	f.submitPasswordInput = password
	return f.nextState, f.submitErr
}
func (f *fakeAuthClient) ListPrivateChats(_ context.Context) ([]domains.ChatSummary, error) {
	return nil, nil
}
func (f *fakeAuthClient) LoadMessages(_ context.Context, _ domains.ChatID, _ int) ([]domains.Message, error) {
	return nil, nil
}
func (f *fakeAuthClient) SendMessage(_ context.Context, _ domains.ChatID, _ string, _ int64) (domains.Message, error) {
	return domains.Message{}, nil
}
func (f *fakeAuthClient) ToggleChatPinned(_ context.Context, _ domains.ChatID, _ bool) error {
	return nil
}
func (f *fakeAuthClient) Updates() <-chan domains.AppEvent { return nil }
func (f *fakeAuthClient) Close() error                     { return nil }

func TestAuthSubmitInput_TrimAndRouteToCode(t *testing.T) {
	fake := &fakeAuthClient{
		session:   domains.AccountSession{Authorized: true},
		nextState: app.AuthState{Step: app.AuthStepCode},
	}
	uc := NewAuth(fake)

	_, _, err := uc.SubmitInput(context.Background(), app.AuthStepCode, " 12345 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.submitCodeInput != "12345" {
		t.Fatalf("expected trimmed code input, got %q", fake.submitCodeInput)
	}
}

func TestAuthSubmitInput_EmptyInputRejected(t *testing.T) {
	uc := NewAuth(&fakeAuthClient{})

	_, _, err := uc.SubmitInput(context.Background(), app.AuthStepPhone, "   ")
	if err == nil {
		t.Fatalf("expected error for empty auth input")
	}
}

func TestAuthBeginQRLogin_PropagatesSessionError(t *testing.T) {
	fake := &fakeAuthClient{
		nextState:  app.AuthState{Step: app.AuthStepQR},
		sessionErr: errors.New("session failed"),
	}
	uc := NewAuth(fake)

	_, _, err := uc.BeginQRLogin(context.Background())
	if err == nil {
		t.Fatalf("expected session error")
	}
	if !fake.beginQRCalled {
		t.Fatalf("expected BeginQRLogin to be called")
	}
}
