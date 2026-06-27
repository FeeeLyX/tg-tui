package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	gotd "github.com/gotd/td/telegram"
	tgauth "github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	qrcode "rsc.io/qr"

	"tg-tui/internal/app"
	"tg-tui/internal/domains"
)

var ErrNotImplemented = errors.New("telegram operation not implemented yet")

type Client struct {
	client  *gotd.Client
	updates chan domains.AppEvent
	readyCh chan struct{}
	stopCh  chan struct{}
	doneCh  chan error

	mu           sync.RWMutex
	runCtx       context.Context
	auth         *tgauth.Client
	authState    app.AuthState
	session      domains.AccountSession
	phone        string
	codeHash     string
	peerByChatID map[domains.ChatID]tg.InputPeerClass
	closed       bool
	started      bool
	verbose      bool
}

func NewClient(config app.Config) *Client {
	service := &Client{
		updates:      make(chan domains.AppEvent, 32),
		readyCh:      make(chan struct{}),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan error, 1),
		verbose:      config.Verbose,
		peerByChatID: make(map[domains.ChatID]tg.InputPeerClass),
		authState: app.AuthState{
			Step: app.AuthStepPhone,
			Hint: "Enter your Telegram phone number to begin login.",
		},
	}

	service.client = gotd.NewClient(config.TelegramAPIID, config.TelegramAPIHash, gotd.Options{
		SessionStorage: &session.FileStorage{Path: config.SessionPath},
		NoUpdates:      true,
	})

	return service
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.mu.Unlock()

	go func() {
		err := c.client.Run(ctx, func(runCtx context.Context) error {
			c.mu.Lock()
			c.runCtx = runCtx
			c.auth = c.client.Auth()
			c.mu.Unlock()

			if err := c.refreshSession(runCtx); err != nil {
				select {
				case c.doneCh <- err:
				default:
				}
				close(c.readyCh)
				return err
			}

			close(c.readyCh)

			select {
			case <-runCtx.Done():
				return nil
			case <-c.stopCh:
				return nil
			}
		})

		select {
		case c.doneCh <- err:
		default:
		}
	}()

	select {
	case <-c.readyCh:
		return nil
	case err := <-c.doneCh:
		if err == nil {
			return errors.New("telegram client exited before initialization")
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) Session(ctx context.Context) (domains.AccountSession, error) {
	if err := c.ensureReady(ctx); err != nil {
		return domains.AccountSession{}, err
	}

	if err := c.refreshSession(c.context()); err != nil {
		return domains.AccountSession{}, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session, nil
}

func (c *Client) AuthState(ctx context.Context) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authState, nil
}

func (c *Client) SubmitPhone(ctx context.Context, phone string) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}
	c.logf("auth SendCode request: phone=%s", maskPhone(phone))

	sentCode, err := c.authClient().SendCode(c.context(), phone, tgauth.SendCodeOptions{})
	if err != nil {
		c.logRPCError("requesting login code", err)
		return app.AuthState{}, mapAuthError("requesting login code", err)
	}
	c.logf("auth SendCode response type: %T", sentCode)

	switch typedCode := sentCode.(type) {
	case *tg.AuthSentCode:
		if timeout, ok := typedCode.GetTimeout(); ok {
			nextType, hasNext := typedCode.GetNextType()
			if hasNext {
				c.logf("auth sent code details: method=%s timeout=%ds next=%s code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), timeout, authCodeTypeLabel(nextType), len(typedCode.PhoneCodeHash))
			} else {
				c.logf("auth sent code details: method=%s timeout=%ds code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), timeout, len(typedCode.PhoneCodeHash))
			}
		} else {
			c.logf("auth sent code details: method=%s code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), len(typedCode.PhoneCodeHash))
		}

		nextState := app.AuthState{
			Step:  app.AuthStepCode,
			Hint:  buildCodeHint(typedCode),
			Phone: phone,
		}

		c.mu.Lock()
		c.phone = phone
		c.codeHash = typedCode.PhoneCodeHash
		c.authState = nextState
		c.session.Phone = phone
		c.mu.Unlock()

		return nextState, nil
	case *tg.AuthSentCodeSuccess:
		c.logf("auth sent code success: already authorized")
		if err := c.refreshSession(c.context()); err != nil {
			return app.AuthState{}, err
		}

		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.authState, nil
	default:
		c.logf("auth unexpected sent code response: %T", sentCode)
		return app.AuthState{}, fmt.Errorf("unexpected auth response type %T", sentCode)
	}
}

func (c *Client) BeginQRLogin(ctx context.Context) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	c.logf("auth QR export request")
	token, err := c.client.QR().Export(c.context())
	if err != nil {
		c.logRPCError("creating QR login token", err)
		return app.AuthState{}, mapAuthError("creating QR login token", err)
	}

	if token.Empty() {
		if err := c.refreshSession(c.context()); err != nil {
			return app.AuthState{}, err
		}

		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.authState, nil
	}

	hint := buildQRHint(token)
	nextState := app.AuthState{
		Step: app.AuthStepQR,
		Hint: hint,
	}

	c.mu.Lock()
	c.authState = nextState
	c.mu.Unlock()

	return nextState, nil
}

func (c *Client) CompleteQRLogin(ctx context.Context) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	c.logf("auth QR import request")
	_, err := c.client.QR().Import(c.context())
	if err != nil {
		if errors.Is(err, tgauth.ErrPasswordAuthNeeded) {
			nextState := app.AuthState{
				Step: app.AuthStepPassword,
				Hint: "Enter your Telegram 2FA password.",
			}

			c.mu.Lock()
			c.authState = nextState
			c.mu.Unlock()

			return nextState, nil
		}

		if rpcErr, ok := tgerr.As(err); ok && rpcErr.Type == "SESSION_PASSWORD_NEEDED" {
			nextState := app.AuthState{
				Step: app.AuthStepPassword,
				Hint: "Enter your Telegram 2FA password.",
			}

			c.mu.Lock()
			c.authState = nextState
			c.mu.Unlock()

			return nextState, nil
		}

		if strings.Contains(err.Error(), "unexpected type *tg.AuthLoginToken") {
			c.mu.RLock()
			current := c.authState
			c.mu.RUnlock()
			if current.Step != app.AuthStepQR {
				current = app.AuthState{Step: app.AuthStepQR}
			}
			if current.Hint == "" {
				current.Hint = "QR is waiting for confirmation. Scan the QR from Telegram app and press Enter to check again."
			}
			return current, errors.New("qr login is not confirmed yet; scan the QR and press Enter again")
		}

		c.logRPCError("confirming QR login", err)
		return app.AuthState{}, mapAuthError("confirming QR login", err)
	}

	if err := c.refreshSession(c.context()); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authState, nil
}

func (c *Client) ResendCode(ctx context.Context) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	phone := c.phone
	codeHash := c.codeHash
	c.mu.RUnlock()

	if phone == "" || codeHash == "" {
		return app.AuthState{}, errors.New("cannot resend code before phone submission")
	}

	c.logf("auth ResendCode request: phone=%s", maskPhone(phone))
	sentCode, err := c.authClient().ResendCode(c.context(), phone, codeHash)
	if err != nil {
		c.logRPCError("resending login code", err)
		return app.AuthState{}, mapAuthError("resending login code", err)
	}
	c.logf("auth ResendCode response type: %T", sentCode)

	switch typedCode := sentCode.(type) {
	case *tg.AuthSentCode:
		if timeout, ok := typedCode.GetTimeout(); ok {
			nextType, hasNext := typedCode.GetNextType()
			if hasNext {
				c.logf("auth resend code details: method=%s timeout=%ds next=%s code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), timeout, authCodeTypeLabel(nextType), len(typedCode.PhoneCodeHash))
			} else {
				c.logf("auth resend code details: method=%s timeout=%ds code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), timeout, len(typedCode.PhoneCodeHash))
			}
		} else {
			c.logf("auth resend code details: method=%s code_hash_len=%d", sentCodeMethodLabel(typedCode.GetType()), len(typedCode.PhoneCodeHash))
		}

		nextState := app.AuthState{
			Step:  app.AuthStepCode,
			Hint:  buildCodeHint(typedCode),
			Phone: phone,
		}

		c.mu.Lock()
		c.codeHash = typedCode.PhoneCodeHash
		c.authState = nextState
		c.mu.Unlock()

		return nextState, nil
	case *tg.AuthSentCodeSuccess:
		c.logf("auth resend success: already authorized")
		if err := c.refreshSession(c.context()); err != nil {
			return app.AuthState{}, err
		}

		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.authState, nil
	default:
		c.logf("auth unexpected resend response: %T", sentCode)
		return app.AuthState{}, fmt.Errorf("unexpected resend response type %T", sentCode)
	}
}

func (c *Client) SubmitCode(ctx context.Context, code string) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	phone := c.phone
	codeHash := c.codeHash
	c.mu.RUnlock()

	if phone == "" || codeHash == "" {
		return app.AuthState{}, errors.New("phone login has not been initiated")
	}

	_, err := c.authClient().SignIn(c.context(), phone, code, codeHash)
	if err != nil {
		c.logRPCError("signing in with code", err)
		if errors.Is(err, tgauth.ErrPasswordAuthNeeded) {
			nextState := app.AuthState{
				Step:  app.AuthStepPassword,
				Hint:  "Enter your Telegram 2FA password.",
				Phone: phone,
			}

			c.mu.Lock()
			c.authState = nextState
			c.mu.Unlock()

			return nextState, nil
		}

		return app.AuthState{}, mapAuthError("signing in with code", err)
	}

	if err := c.refreshSession(c.context()); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authState, nil
}

func (c *Client) SubmitPassword(ctx context.Context, password string) (app.AuthState, error) {
	if err := c.ensureReady(ctx); err != nil {
		return app.AuthState{}, err
	}

	_, err := c.authClient().Password(c.context(), password)
	if err != nil {
		c.logRPCError("verifying 2FA password", err)
		return app.AuthState{}, mapAuthError("verifying 2FA password", err)
	}

	if err := c.refreshSession(c.context()); err != nil {
		return app.AuthState{}, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authState, nil
}

func (c *Client) ListPrivateChats(ctx context.Context) ([]domains.ChatSummary, error) {
	if err := c.ensureReady(ctx); err != nil {
		return nil, err
	}

	if err := c.refreshSession(c.context()); err != nil {
		return nil, err
	}

	c.mu.RLock()
	authorized := c.session.Authorized
	c.mu.RUnlock()
	if !authorized {
		return nil, errors.New("telegram session is not authorized")
	}

	raw := tg.NewClient(c.client)
	iter := query.GetDialogs(raw).BatchSize(100).Iter()

	chats := make([]domains.ChatSummary, 0, 64)
	peers := make(map[domains.ChatID]tg.InputPeerClass)
	for iter.Next(c.context()) {
		elem := iter.Value()
		if elem.Deleted() {
			continue
		}

		peerUser, ok := elem.Dialog.GetPeer().(*tg.PeerUser)
		if !ok {
			// MVP scope: private chats only.
			continue
		}

		user, ok := elem.Entities.User(peerUser.UserID)
		if !ok {
			continue
		}

		lastText, lastAt := extractLastMessage(elem.Last)
		chat := domains.ChatSummary{
			ID:              domains.ChatID(user.ID),
			Title:           privateChatTitle(user),
			LastMessageText: lastText,
			LastMessageAt:   lastAt,
			UnreadCount:     dialogUnreadCount(elem.Dialog),
			IsOnline:        isUserOnline(user),
		}
		peers[chat.ID] = elem.Peer
		chats = append(chats, chat)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("list dialogs: %w", err)
	}

	sort.SliceStable(chats, func(i, j int) bool {
		return chats[i].LastMessageAt.After(chats[j].LastMessageAt)
	})

	c.mu.Lock()
	c.peerByChatID = peers
	c.mu.Unlock()

	return chats, nil
}

func (c *Client) LoadMessages(ctx context.Context, chatID domains.ChatID, limit int) ([]domains.Message, error) {
	if err := c.ensureReady(ctx); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	if err := c.refreshSession(c.context()); err != nil {
		return nil, err
	}

	c.mu.RLock()
	authorized := c.session.Authorized
	peerInput, ok := c.peerByChatID[chatID]
	c.mu.RUnlock()
	if !authorized {
		return nil, errors.New("telegram session is not authorized")
	}

	if !ok {
		if _, err := c.ListPrivateChats(ctx); err != nil {
			return nil, err
		}

		c.mu.RLock()
		peerInput, ok = c.peerByChatID[chatID]
		c.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("chat %d is unavailable or no longer accessible", chatID)
		}
	}

	raw := tg.NewClient(c.client)
	iter := query.Messages(raw).GetHistory(peerInput).BatchSize(min(limit, 100)).Iter()

	messages := make([]domains.Message, 0, limit)
	for iter.Next(c.context()) {
		elem := iter.Value()
		messages = append(messages, mapMessage(elem.Msg, chatID, elem.Entities))
		if len(messages) >= limit {
			break
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	// Render oldest to newest in the UI.
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	return messages, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID domains.ChatID, text string, replyToMessageID int64) (domains.Message, error) {
	if err := c.ensureReady(ctx); err != nil {
		return domains.Message{}, err
	}

	body := strings.TrimSpace(text)
	if body == "" {
		return domains.Message{}, errors.New("message text cannot be empty")
	}

	if err := c.refreshSession(c.context()); err != nil {
		return domains.Message{}, err
	}

	c.mu.RLock()
	authorized := c.session.Authorized
	peerInput, ok := c.peerByChatID[chatID]
	c.mu.RUnlock()
	if !authorized {
		return domains.Message{}, errors.New("telegram session is not authorized")
	}

	if !ok {
		if _, err := c.ListPrivateChats(ctx); err != nil {
			return domains.Message{}, err
		}

		c.mu.RLock()
		peerInput, ok = c.peerByChatID[chatID]
		c.mu.RUnlock()
		if !ok {
			return domains.Message{}, fmt.Errorf("chat %d is unavailable or no longer accessible", chatID)
		}
	}

	randomID := time.Now().UnixNano()
	req := &tg.MessagesSendMessageRequest{
		Peer:     peerInput,
		Message:  body,
		RandomID: randomID,
	}
	if replyToMessageID > 0 {
		req.ReplyTo = &tg.InputReplyToMessage{ReplyToMsgID: int(replyToMessageID)}
	}

	raw := tg.NewClient(c.client)
	_, err := raw.MessagesSendMessage(c.context(), req)
	if err != nil {
		c.logRPCError("sending message", err)
		return domains.Message{}, mapAuthError("sending message", err)
	}

	return domains.Message{
		ID:         randomID,
		ChatID:     chatID,
		SenderName: "You",
		Text:       body,
		Direction:  domains.MessageDirectionOutgoing,
		SentAt:     time.Now(),
	}, nil
}

func (c *Client) Updates() <-chan domains.AppEvent {
	return c.updates
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.stopCh)
	c.mu.Unlock()

	err := <-c.doneCh
	close(c.updates)
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

func (c *Client) ensureReady(ctx context.Context) error {
	select {
	case <-c.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) context() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runCtx
}

func (c *Client) authClient() *tgauth.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.auth
}

func (c *Client) refreshSession(ctx context.Context) error {
	status, err := c.authClient().Status(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if status.Authorized {
		c.session = domains.AccountSession{
			Authorized: true,
			Phone:      c.phone,
			UpdatedAt:  time.Now(),
		}
		c.authState = app.AuthState{
			Step:  app.AuthStepCode,
			Hint:  "Authorized. Chat sync is the next implementation slice.",
			Phone: c.phone,
		}
		return nil
	}

	c.session = domains.AccountSession{
		Authorized: false,
		Phone:      c.phone,
		UpdatedAt:  time.Now(),
	}
	if c.authState.Step == "" {
		c.authState = app.AuthState{
			Step: app.AuthStepPhone,
			Hint: "Enter your Telegram phone number to begin login.",
		}
	}

	return nil
}

func buildCodeHint(code *tg.AuthSentCode) string {
	method := sentCodeMethodLabel(code.GetType())
	hint := fmt.Sprintf("Telegram sent the login code via %s.", method)
	if timeout, ok := code.GetTimeout(); ok && timeout > 0 {
		nextType, hasNext := code.GetNextType()
		if hasNext {
			hint = fmt.Sprintf("Telegram sent the login code via %s. If not received in %ds, fallback is %s.", method, timeout, authCodeTypeLabel(nextType))
		} else {
			hint = fmt.Sprintf("Telegram sent the login code via %s. Timeout: %ds.", method, timeout)
		}
	}

	return hint
}

func buildQRHint(token qrlogin.Token) string {
	code, err := qrcode.Encode(token.URL(), qrcode.M)
	if err != nil {
		return fmt.Sprintf("Scan this link in Telegram app and press Enter after approval: %s", token.URL())
	}

	var lines []string
	lines = append(lines, "Scan this QR from Telegram app: Settings -> Devices -> Link Desktop Device")
	lines = append(lines, "")

	margin := 2
	for y := -margin; y < code.Size+margin; y++ {
		var row strings.Builder
		for x := -margin; x < code.Size+margin; x++ {
			black := x >= 0 && y >= 0 && x < code.Size && y < code.Size && code.Black(x, y)
			if black {
				row.WriteString("██")
			} else {
				row.WriteString("  ")
			}
		}
		lines = append(lines, row.String())
	}

	if expires := token.Expires(); !expires.IsZero() {
		remaining := time.Until(expires).Round(time.Second)
		if remaining < 0 {
			remaining = 0
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Token expires in about %s. Press g to regenerate if expired.", remaining))
	}

	lines = append(lines, "Press Enter after scanning to complete login.")
	return strings.Join(lines, "\n")
}

func sentCodeMethodLabel(value tg.AuthSentCodeTypeClass) string {
	switch value.(type) {
	case *tg.AuthSentCodeTypeApp:
		return "Telegram app"
	case *tg.AuthSentCodeTypeSMS, *tg.AuthSentCodeTypeSMSWord, *tg.AuthSentCodeTypeSMSPhrase, *tg.AuthSentCodeTypeFragmentSMS, *tg.AuthSentCodeTypeFirebaseSMS:
		return "SMS"
	case *tg.AuthSentCodeTypeCall:
		return "phone call"
	case *tg.AuthSentCodeTypeFlashCall:
		return "flash call"
	case *tg.AuthSentCodeTypeMissedCall:
		return "missed call"
	case *tg.AuthSentCodeTypeEmailCode:
		return "email"
	default:
		if value == nil {
			return "unknown channel"
		}
		return value.TypeName()
	}
}

func authCodeTypeLabel(value tg.AuthCodeTypeClass) string {
	switch value.(type) {
	case *tg.AuthCodeTypeSMS, *tg.AuthCodeTypeFragmentSMS:
		return "SMS"
	case *tg.AuthCodeTypeCall:
		return "phone call"
	case *tg.AuthCodeTypeFlashCall:
		return "flash call"
	case *tg.AuthCodeTypeMissedCall:
		return "missed call"
	default:
		if value == nil {
			return "unknown channel"
		}
		return value.TypeName()
	}
}

func (c *Client) logRPCError(action string, err error) {
	if wait, ok := tgerr.AsFloodWait(err); ok {
		c.logf("auth %s flood-wait: %s", action, wait.Round(time.Second))
	}

	if rpcErr, ok := tgerr.As(err); ok {
		c.logf("auth %s rpc error: code=%d type=%s message=%s arg=%d", action, rpcErr.Code, rpcErr.Type, rpcErr.Message, rpcErr.Argument)
		return
	}

	c.logf("auth %s generic error: %v", action, err)
}

func (c *Client) logf(format string, args ...any) {
	if !c.verbose {
		return
	}

	line := fmt.Sprintf("%s [INFO] tg-client: %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	_, _ = os.Stderr.WriteString(line)
}

func maskPhone(phone string) string {
	trimmed := strings.TrimSpace(phone)
	if len(trimmed) <= 4 {
		return "****"
	}

	return trimmed[:2] + strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-2:]
}

func mapAuthError(action string, err error) error {
	if wait, ok := tgerr.AsFloodWait(err); ok {
		return fmt.Errorf("telegram rate limit while %s: wait %s before retrying", action, wait.Round(time.Second))
	}

	rpcErr, ok := tgerr.As(err)
	if !ok {
		return err
	}

	switch rpcErr.Type {
	case "PHONE_NUMBER_INVALID":
		return errors.New("invalid phone number format: include country code, e.g. +15551234567")
	case "PHONE_NUMBER_BANNED":
		return errors.New("this phone number is banned by Telegram")
	case "PHONE_NUMBER_FLOOD":
		return errors.New("too many login attempts for this phone number; wait and retry later")
	case "PHONE_PASSWORD_FLOOD":
		return errors.New("too many 2FA attempts; wait and retry later")
	case "PHONE_CODE_INVALID":
		return errors.New("invalid login code")
	case "PHONE_CODE_EXPIRED":
		return errors.New("login code expired, request a new code")
	case "SEND_CODE_UNAVAILABLE":
		return errors.New("telegram does not allow another code delivery for this attempt; wait and retry later, or initiate login from an official app first")
	case "SESSION_PASSWORD_NEEDED":
		return errors.New("2FA password is required")
	case "PASSWORD_HASH_INVALID":
		return errors.New("invalid 2FA password")
	case "API_ID_INVALID":
		return errors.New("telegram app credentials are invalid (API_ID/API_HASH)")
	case "API_ID_PUBLISHED_FLOOD":
		return errors.New("shared app credentials are temporarily limited by Telegram; retry later or switch to your own credentials")
	}

	if strings.Contains(rpcErr.Message, "FLOOD") {
		return fmt.Errorf("telegram temporarily rate-limited this action while %s: %s", action, rpcErr.Message)
	}

	if rpcErr.Code != 0 || rpcErr.Message != "" {
		return fmt.Errorf("telegram error while %s: %s (code %d)", action, rpcErr.Message, rpcErr.Code)
	}

	return err
}

func privateChatTitle(user *tg.User) string {
	if user == nil {
		return "Unknown"
	}

	if user.Self {
		return "Saved Messages"
	}

	fullName := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if fullName != "" {
		return fullName
	}

	if user.Username != "" {
		return "@" + user.Username
	}

	return fmt.Sprintf("User %d", user.ID)
}

func isUserOnline(user *tg.User) bool {
	if user == nil {
		return false
	}

	_, ok := user.Status.(*tg.UserStatusOnline)
	return ok
}

func extractLastMessage(msg tg.NotEmptyMessage) (string, time.Time) {
	if msg == nil {
		return "", time.Time{}
	}

	timestamp := time.Unix(int64(msg.GetDate()), 0)
	switch typed := msg.(type) {
	case *tg.Message:
		return strings.TrimSpace(typed.Message), timestamp
	case *tg.MessageService:
		return "[service message]", timestamp
	default:
		return "", timestamp
	}
}

func mapMessage(msg tg.NotEmptyMessage, chatID domains.ChatID, entities peer.Entities) domains.Message {
	result := domains.Message{
		ID:     int64(msg.GetID()),
		ChatID: chatID,
		SentAt: time.Unix(int64(msg.GetDate()), 0),
	}

	if msg.GetOut() {
		result.Direction = domains.MessageDirectionOutgoing
	} else {
		result.Direction = domains.MessageDirectionIncoming
	}

	if fromID, ok := msg.GetFromID(); ok {
		result.SenderName = senderName(fromID, entities)
	}

	switch typed := msg.(type) {
	case *tg.Message:
		result.Text = strings.TrimSpace(typed.GetMessage())
		if result.Text == "" {
			result.Text = "[media/message without text]"
		}
	case *tg.MessageService:
		result.Text = "[service message]"
	default:
		result.Text = "[unsupported message type]"
	}

	return result
}

func senderName(from tg.PeerClass, entities peer.Entities) string {
	switch typed := from.(type) {
	case *tg.PeerUser:
		user, ok := entities.User(typed.UserID)
		if !ok {
			return ""
		}
		return privateChatTitle(user)
	case *tg.PeerChat:
		chat, ok := entities.Chat(typed.ChatID)
		if ok && strings.TrimSpace(chat.Title) != "" {
			return chat.Title
		}
	case *tg.PeerChannel:
		channel, ok := entities.Channel(typed.ChannelID)
		if ok && strings.TrimSpace(channel.Title) != "" {
			return channel.Title
		}
	}

	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func dialogUnreadCount(dialog tg.DialogClass) int {
	if typed, ok := dialog.(*tg.Dialog); ok {
		return typed.UnreadCount
	}

	return 0
}
