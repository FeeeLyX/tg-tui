package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"tg-tui/internal/app"
	"tg-tui/internal/domains"
)

var (
	panelStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	headerStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	errorStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	mutedStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	incomingNameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	outgoingNameStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	selectedMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11")).Bold(true)
)

type Model struct {
	state                 app.State
	client                app.TelegramClient
	width                 int
	height                int
	authInput             textinput.Model
	composeInput          textinput.Model
	messageView           bool
	messageScroll         int
	messageLimitByChat    map[domains.ChatID]int
	selectedMessageByChat map[domains.ChatID]int
	replyToMessageByChat  map[domains.ChatID]int64
}

type authResultMsg struct {
	state   app.AuthState
	session domains.AccountSession
	err     error
}

type chatsLoadedMsg struct {
	chats  []domains.ChatSummary
	err    error
	silent bool
}

type messagesLoadedMsg struct {
	chatID         domains.ChatID
	messages       []domains.Message
	err            error
	limit          int
	preserveTop    bool
	previousCount  int
	preserveScroll bool
	silent         bool
}

type refreshTickMsg struct{}

type messageSentMsg struct {
	chatID  domains.ChatID
	message domains.Message
	err     error
}

func NewModel(state app.State, client app.TelegramClient) Model {
	authInput := textinput.New()
	authInput.Placeholder = "+123456789"
	authInput.Focus()
	authInput.CharLimit = 64
	authInput.Width = 28

	composeInput := textinput.New()
	composeInput.Placeholder = "Type a message"
	composeInput.CharLimit = 4096
	composeInput.Width = 48
	composeInput.Blur()

	return Model{
		state:                 state,
		client:                client,
		authInput:             authInput,
		composeInput:          composeInput,
		messageLimitByChat:    map[domains.ChatID]int{},
		selectedMessageByChat: map[domains.ChatID]int{},
		replyToMessageByChat:  map[domains.ChatID]int64{},
	}
}

func (m Model) Init() tea.Cmd {
	if m.state.Session.Authorized && m.client != nil {
		m.state.Status = "Syncing private chats"
		return tea.Batch(textinput.Blink, m.loadPrivateChats(), m.scheduleRefresh())
	}

	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil
	case refreshTickMsg:
		if !m.state.Session.Authorized || m.client == nil {
			return m, m.scheduleRefresh()
		}

		cmds := []tea.Cmd{m.loadPrivateChatsSilent(), m.scheduleRefresh()}
		if m.messageView && m.state.ActiveChatID != 0 {
			cmds = append(cmds, m.refreshActiveMessages())
		}
		return m, tea.Batch(cmds...)
	case tea.MouseMsg:
		if !m.state.Session.Authorized {
			return m, nil
		}
		if typed.Action != tea.MouseActionPress || typed.Button != tea.MouseButtonLeft {
			return m, nil
		}
		return m, m.handleMouseClick(typed)
	case authResultMsg:
		if typed.err != nil {
			if typed.state.Step != "" {
				m.state.AuthState = typed.state
			}
			m.state.Error = typed.err
			m.state.Status = "Telegram auth failed"
			return m, nil
		}

		m.state.Error = nil
		m.state.AuthState = typed.state
		m.state.Session = typed.session
		m.messageView = false
		m.messageScroll = 0
		if typed.session.Authorized {
			m.authInput.Blur()
			m.composeInput.Focus()
		} else {
			m.authInput.Focus()
			m.composeInput.Blur()
		}
		if typed.session.Authorized {
			m.state.Status = "Authorized. Syncing private chats"
		} else {
			m.state.Status = "Awaiting Telegram auth input"
		}

		m.authInput.SetValue("")
		m.authInput.Placeholder = authPlaceholder(m.state.AuthState.Step)
		if typed.state.Step == app.AuthStepPassword {
			m.authInput.EchoMode = textinput.EchoPassword
			m.authInput.EchoCharacter = '*'
		} else {
			m.authInput.EchoMode = textinput.EchoNormal
		}

		if typed.session.Authorized && m.client != nil {
			return m, m.loadPrivateChats()
		}

		return m, nil
	case chatsLoadedMsg:
		if typed.err != nil {
			if !typed.silent {
				m.state.Error = typed.err
				m.state.Status = "Authorized, but chat sync failed"
			}
			return m, nil
		}

		if !typed.silent {
			m.state.Error = nil
		}
		previousActive := m.state.ActiveChatID
		m.state.Chats = typed.chats
		if previousActive != 0 {
			stillPresent := false
			for _, chat := range typed.chats {
				if chat.ID == previousActive {
					stillPresent = true
					break
				}
			}
			if stillPresent {
				m.state.ActiveChatID = previousActive
			} else {
				m.state.ActiveChatID = 0
			}
		}
		if m.state.ActiveChatID == 0 && len(typed.chats) > 0 {
			m.state.ActiveChatID = typed.chats[0].ID
		}

		if typed.silent {
			return m, nil
		}

		if len(typed.chats) == 0 {
			m.state.Status = "Authorized. No private chats found"
			return m, nil
		} else {
			m.state.Status = fmt.Sprintf("Loaded %d private chats", len(typed.chats))
			return m, nil
		}
	case messagesLoadedMsg:
		if typed.chatID != m.state.ActiveChatID {
			return m, nil
		}

		if typed.err != nil {
			if !typed.silent {
				m.state.Error = typed.err
				m.state.Status = "Failed to load messages"
			}
			return m, nil
		}

		if !typed.silent {
			m.state.Error = nil
		}
		previousMessages := m.state.MessagesByChat[typed.chatID]
		previousSelectedIndex, hadSelection := m.selectedMessageByChat[typed.chatID]
		var previousSelectedID int64
		if hadSelection && previousSelectedIndex >= 0 && previousSelectedIndex < len(previousMessages) {
			previousSelectedID = previousMessages[previousSelectedIndex].ID
		}
		m.state.MessagesByChat[typed.chatID] = typed.messages
		m.messageLimitByChat[typed.chatID] = typed.limit
		if current, ok := m.selectedMessageByChat[typed.chatID]; ok && typed.preserveTop {
			delta := len(typed.messages) - typed.previousCount
			if delta > 0 {
				m.selectedMessageByChat[typed.chatID] = current + delta
			}
		}
		if typed.preserveScroll && hadSelection {
			restored := false
			if previousSelectedID != 0 {
				for i := range typed.messages {
					if typed.messages[i].ID == previousSelectedID {
						m.selectedMessageByChat[typed.chatID] = i
						restored = true
						break
					}
				}
			}

			if !restored {
				if previousSelectedIndex < 0 {
					previousSelectedIndex = 0
				}
				if len(typed.messages) == 0 {
					m.selectedMessageByChat[typed.chatID] = 0
				} else if previousSelectedIndex >= len(typed.messages) {
					m.selectedMessageByChat[typed.chatID] = len(typed.messages) - 1
				} else {
					m.selectedMessageByChat[typed.chatID] = previousSelectedIndex
				}
			}
		} else if _, ok := m.selectedMessageByChat[typed.chatID]; !ok || !typed.preserveTop {
			if len(typed.messages) > 0 {
				m.selectedMessageByChat[typed.chatID] = len(typed.messages) - 1
			} else {
				m.selectedMessageByChat[typed.chatID] = 0
			}
		}
		if len(typed.messages) > 0 && m.selectedMessageByChat[typed.chatID] >= len(typed.messages) {
			m.selectedMessageByChat[typed.chatID] = len(typed.messages) - 1
		}
		if typed.preserveTop {
			delta := len(typed.messages) - typed.previousCount
			if delta > 0 {
				m.messageScroll += delta
			}
			if maxScroll := m.maxMessageScroll(); m.messageScroll > maxScroll {
				m.messageScroll = maxScroll
			}
		} else if typed.preserveScroll {
			if maxScroll := m.maxMessageScroll(); m.messageScroll > maxScroll {
				m.messageScroll = maxScroll
			}
		} else {
			m.messageScroll = 0
		}
		m.messageView = true
		if typed.silent {
			return m, nil
		}
		if typed.preserveTop {
			m.state.Status = fmt.Sprintf("Loaded %d older messages", max(0, len(typed.messages)-typed.previousCount))
			return m, nil
		}
		m.state.Status = fmt.Sprintf("Loaded %d messages", len(typed.messages))
		return m, nil
	case messageSentMsg:
		if typed.chatID != m.state.ActiveChatID {
			return m, nil
		}

		if typed.err != nil {
			m.state.Error = typed.err
			m.state.Status = "Failed to send message"
			return m, nil
		}

		m.state.Error = nil
		m.state.MessagesByChat[typed.chatID] = append(m.state.MessagesByChat[typed.chatID], typed.message)
		m.selectedMessageByChat[typed.chatID] = len(m.state.MessagesByChat[typed.chatID]) - 1
		delete(m.replyToMessageByChat, typed.chatID)
		m.composeInput.SetValue("")
		m.composeInput.Focus()
		m.messageScroll = 0
		m.state.Status = "Message sent"
		return m, nil
	case tea.KeyMsg:
		if isMouseEscapeKey(typed) {
			return m, nil
		}

		switch typed.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.state.Session.Authorized {
				if m.messageView {
					if m.scrollMessages(1) {
						m.state.Status = "Scrolled up"
						return m, nil
					}

					if m.canLoadOlderMessages() {
						m.state.Status = "Loading older messages"
						m.state.Error = nil
						return m, m.loadMoreMessagesForActiveChat()
					}

					m.state.Status = "Reached oldest loaded message"
					return m, nil
				}

				if m.selectRelativeChat(-1) {
					m.state.Status = "Chat selected. Press Enter to load messages"
					m.state.Error = nil
					m.messageScroll = 0
				}
				return m, nil
			}
		case "down":
			if m.state.Session.Authorized {
				if m.messageView {
					if m.scrollMessages(-1) {
						m.state.Status = "Scrolled down"
					} else {
						m.state.Status = "Reached newest loaded message"
					}
					return m, nil
				}

				if m.selectRelativeChat(1) {
					m.state.Status = "Chat selected. Press Enter to load messages"
					m.state.Error = nil
					m.messageScroll = 0
				}
				return m, nil
			}
		case "esc", "left":
			if m.state.Session.Authorized && m.messageView {
				m.messageView = false
				m.messageScroll = 0
				m.composeInput.Blur()
				m.state.Status = "Back to chat selection"
				return m, nil
			}
		case "right":
			if m.state.Session.Authorized && !m.messageView {
				if m.state.ActiveChatID == 0 || m.client == nil {
					return m, nil
				}

				m.state.Status = "Loading messages"
				m.state.Error = nil
				m.composeInput.Focus()
				return m, m.loadMessagesForActiveChat()
			}
		case "g":
			if !m.state.Session.Authorized {
				if m.client == nil {
					m.state.Error = fmt.Errorf("telegram credentials are unavailable")
					m.state.Status = "Telegram credentials required"
					return m, nil
				}

				m.state.Status = "Generating Telegram QR login"
				m.state.Error = nil
				return m, m.beginQRLogin()
			}
		case "ctrl+r":
			if m.state.Session.Authorized && m.messageView {
				selected, ok := m.selectedCurrentMessage()
				if !ok {
					m.state.Status = "Select a message to reply"
					return m, nil
				}
				m.replyToMessageByChat[m.state.ActiveChatID] = selected.ID
				m.state.Status = fmt.Sprintf("Replying to: %s", previewReplyText(selected.Text))
				return m, nil
			}

			if !m.state.Session.Authorized && m.state.AuthState.Step == app.AuthStepCode {
				if m.client == nil {
					m.state.Error = fmt.Errorf("telegram client is not initialized")
					return m, nil
				}

				m.state.Status = "Requesting another Telegram login code"
				m.state.Error = nil
				return m, m.resendCode()
			}
		case "enter":
			if m.state.Session.Authorized {
				if m.state.ActiveChatID == 0 || m.client == nil {
					return m, nil
				}

				if m.messageView {
					m.composeInput.Focus()
					text := strings.TrimSpace(m.composeInput.Value())
					if text == "" {
						m.state.Status = "Type a message to send"
						return m, nil
					}

					m.state.Status = "Sending message"
					m.state.Error = nil
					replyTo := m.replyToMessageByChat[m.state.ActiveChatID]
					return m, m.sendMessage(m.state.ActiveChatID, text, replyTo)
				}

				m.state.Status = "Loading messages"
				m.state.Error = nil
				m.composeInput.Focus()
				return m, m.loadMessagesForActiveChat()
			}

			if !m.state.Session.Authorized {
				if m.client == nil {
					if m.state.Error == nil {
						m.state.Error = fmt.Errorf("telegram credentials are unavailable")
					}
					m.state.Status = "Telegram credentials required"
					return m, nil
				}

				if m.state.AuthState.Step == app.AuthStepQR {
					m.state.Status = "Checking Telegram QR confirmation"
					m.state.Error = nil
					return m, m.completeQRLogin()
				}

				value := strings.TrimSpace(m.authInput.Value())
				if value == "" {
					return m, nil
				}

				m.state.Status = "Submitting Telegram auth input"
				m.state.Error = nil
				return m, m.submitAuth(value)
			}
		case "ctrl+u":
			if m.state.Session.Authorized && m.messageView {
				delete(m.replyToMessageByChat, m.state.ActiveChatID)
				m.state.Status = "Reply target cleared"
				return m, nil
			}
		case "ctrl+up":
			if m.state.Session.Authorized && m.messageView {
				if m.moveMessageSelection(-1) {
					m.state.Status = "Message selected"
				} else {
					m.state.Status = "Reached oldest message"
				}
				return m, nil
			}
		case "ctrl+down":
			if m.state.Session.Authorized && m.messageView {
				if m.moveMessageSelection(1) {
					m.state.Status = "Message selected"
				} else {
					m.state.Status = "Reached newest message"
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if !m.state.Session.Authorized {
		m.authInput, cmd = m.authInput.Update(msg)
		return m, cmd
	}

	m.composeInput, cmd = m.composeInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 120
	}

	if m.height == 0 {
		m.height = 32
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render("tg-tui"),
		m.renderBody(),
		m.renderStatus(),
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m Model) renderBody() string {
	if !m.state.Session.Authorized {
		return m.renderAuth()
	}

	leftWidth := max(30, m.width/3)
	rightWidth := max(40, m.width-leftWidth-8)
	panelHeight := max(12, m.height-6)
	contentRows := max(1, panelHeight-2)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(leftWidth).Height(panelHeight).Render(m.renderChats(contentRows)),
		panelStyle.Width(rightWidth).Height(panelHeight).Render(m.renderMessages(contentRows)),
	)
}

func (m Model) renderAuth() string {
	stepTitle := map[app.AuthStep]string{
		app.AuthStepPhone:    "Enter your phone number",
		app.AuthStepCode:     "Enter the login code",
		app.AuthStepQR:       "Scan Telegram QR login",
		app.AuthStepPassword: "Enter your 2FA password",
	}[m.state.AuthState.Step]

	var lines []string
	lines = append(lines, headerStyle.Render(stepTitle))
	lines = append(lines, mutedStyle.Render("Telegram credentials are loaded from TG_TUI_API_ID/TG_TUI_API_HASH."))
	if m.state.CredentialSummary != "" {
		lines = append(lines, mutedStyle.Render(m.state.CredentialSummary))
	}
	if m.state.AuthState.Hint != "" {
		lines = append(lines, mutedStyle.Render(m.state.AuthState.Hint))
	}
	if m.state.CredentialNotice != "" {
		lines = append(lines, errorStyle.Render(m.state.CredentialNotice))
	}
	if m.state.Error != nil {
		lines = append(lines, errorStyle.Render(m.state.Error.Error()))
	}
	lines = append(lines, "")
	lines = append(lines, m.authInput.View())
	lines = append(lines, "")
	if m.state.AuthState.Step == app.AuthStepQR {
		lines = append(lines, mutedStyle.Render("Press g to regenerate QR. Press Enter after scanning to verify login."))
	} else if m.state.AuthState.Step == app.AuthStepCode {
		lines = append(lines, mutedStyle.Render("Press Enter to submit code. Press r to request another code."))
	} else {
		lines = append(lines, mutedStyle.Render("Press Enter to submit. Press g to switch to QR login."))
	}

	return panelStyle.Width(max(60, m.width-8)).Render(strings.Join(lines, "\n"))
}

func (m Model) renderChats(maxRows int) string {
	lines := []string{headerStyle.Render("Chats")}
	if len(m.state.Chats) == 0 {
		lines = append(lines, mutedStyle.Render("No chats loaded yet."))
		return strings.Join(clampLines(lines, maxRows), "\n")
	}

	currentIndex := m.activeChatIndex()
	if currentIndex == -1 {
		currentIndex = 0
		m.state.ActiveChatID = m.state.Chats[0].ID
	}

	visibleRows := max(1, maxRows-3)
	leftWidth := max(30, m.width/3)
	contentWidth := max(12, leftWidth-6)
	start := currentIndex - visibleRows/2
	if start < 0 {
		start = 0
	}
	if start+visibleRows > len(m.state.Chats) {
		start = len(m.state.Chats) - visibleRows
		if start < 0 {
			start = 0
		}
	}
	end := start + visibleRows
	if end > len(m.state.Chats) {
		end = len(m.state.Chats)
	}

	if start > 0 {
		lines = append(lines, mutedStyle.Render("..."))
	}

	for i := start; i < end; i++ {
		chat := m.state.Chats[i]
		prefix := "  "
		if chat.ID == m.state.ActiveChatID {
			prefix = "> "
		}

		preview := oneLine(chat.LastMessageText)
		if preview != "" {
			preview = " - " + truncateRunes(preview, 36)
		}

		unread := ""
		if chat.UnreadCount > 0 {
			unread = " (" + strconv.Itoa(chat.UnreadCount) + ")"
		}

		entry := truncateDisplayWidth(chat.Title+unread+preview, contentWidth)
		lines = append(lines, fmt.Sprintf("%s%s", prefix, entry))
	}

	if end < len(m.state.Chats) {
		lines = append(lines, mutedStyle.Render("..."))
	}

	return strings.Join(clampLines(lines, maxRows), "\n")
}

func (m Model) renderMessages(maxRows int) string {
	lines := []string{headerStyle.Render("Messages")}
	rightWidth := max(40, m.width-max(30, m.width/3)-8)
	messageWidth := max(20, rightWidth-6)

	messages, loaded := m.state.MessagesByChat[m.state.ActiveChatID]
	hasScrollLine := loaded && len(messages) > 1
	footerRows := 4 // blank + compose label + input + mode hint
	if hasScrollLine {
		footerRows++
	}
	messageRows := max(0, maxRows-1-footerRows)

	if !loaded {
		if m.state.ActiveChatID == 0 {
			lines = append(lines, mutedStyle.Render("Select a chat first."))
		} else {
			lines = append(lines, mutedStyle.Render("Press Enter to load messages for selected chat."))
		}
	} else if len(messages) == 0 {
		lines = append(lines, mutedStyle.Render("No recent messages in this chat."))
	} else {
		for _, line := range m.buildMessageViewLines(messages, messageWidth, messageRows) {
			lines = append(lines, line)
		}

		if hasScrollLine {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("Scroll: %d/%d", m.messageScroll, m.maxMessageScroll())))
		}
	}

	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Compose"))
	lines = append(lines, m.composeInput.View())
	if m.messageView {
		if replyID, ok := m.replyToMessageByChat[m.state.ActiveChatID]; ok && replyID > 0 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("Replying to message #%d (Ctrl+r sets target from selected, Ctrl+u clears)", replyID)))
		} else {
			lines = append(lines, mutedStyle.Render("Reply: click/select message then press Ctrl+r to set target"))
		}
		lines = append(lines, mutedStyle.Render("Message mode: Up/Down scroll, Ctrl+Up/Down select, Enter send, Esc/Left back"))
	} else {
		lines = append(lines, mutedStyle.Render("Chat mode: Up/Down to select, Enter/Right to open chat"))
	}

	return strings.Join(clampLines(lines, maxRows), "\n")
}

func (m Model) renderStatus() string {
	status := m.state.Status
	if status == "" {
		status = "Idle"
	}

	line := mutedStyle.Render(status)
	if m.state.Error != nil {
		line = errorStyle.Render(m.state.Error.Error())
	}

	return lipgloss.NewStyle().PaddingTop(1).Render(line)
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func authPlaceholder(step app.AuthStep) string {
	switch step {
	case app.AuthStepCode:
		return "12345"
	case app.AuthStepPassword:
		return "2FA password"
	case app.AuthStepQR:
		return "Press g to generate QR"
	default:
		return "+123456789"
	}
}

func (m Model) submitAuth(value string) tea.Cmd {
	step := m.state.AuthState.Step
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()

		var (
			nextState app.AuthState
			err       error
		)

		switch step {
		case app.AuthStepQR:
			nextState, err = client.CompleteQRLogin(ctx)
		case app.AuthStepCode:
			nextState, err = client.SubmitCode(ctx, value)
		case app.AuthStepPassword:
			nextState, err = client.SubmitPassword(ctx, value)
		default:
			nextState, err = client.SubmitPhone(ctx, value)
		}

		session := m.state.Session
		if err == nil {
			session, err = client.Session(ctx)
		}

		return authResultMsg{state: nextState, session: session, err: err}
	}
}

func (m Model) resendCode() tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		nextState, err := client.ResendCode(ctx)

		session := m.state.Session
		if err == nil {
			session, err = client.Session(ctx)
		}

		return authResultMsg{state: nextState, session: session, err: err}
	}
}

func (m Model) beginQRLogin() tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		nextState, err := client.BeginQRLogin(ctx)

		session := m.state.Session
		if err == nil {
			session, err = client.Session(ctx)
		}

		return authResultMsg{state: nextState, session: session, err: err}
	}
}

func (m Model) completeQRLogin() tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		nextState, err := client.CompleteQRLogin(ctx)

		session := m.state.Session
		if err == nil {
			session, err = client.Session(ctx)
		}

		return authResultMsg{state: nextState, session: session, err: err}
	}
}

func ApplyCachedChats(state app.State, chats []domains.ChatSummary) app.State {
	state.Chats = chats
	if state.ActiveChatID == 0 && len(chats) > 0 {
		state.ActiveChatID = chats[0].ID
	}
	return state
}

func ApplyCachedMessages(state app.State, chatID domains.ChatID, messages []domains.Message) app.State {
	state.MessagesByChat[chatID] = messages
	return state
}

func (m Model) loadPrivateChats() tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		chats, err := client.ListPrivateChats(ctx)
		return chatsLoadedMsg{chats: chats, err: err, silent: false}
	}
}

func (m Model) loadPrivateChatsSilent() tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		chats, err := client.ListPrivateChats(ctx)
		return chatsLoadedMsg{chats: chats, err: err, silent: true}
	}
}

func (m Model) loadMessagesForActiveChat() tea.Cmd {
	client := m.client
	chatID := m.state.ActiveChatID
	limit := m.messageLimit(chatID)

	return func() tea.Msg {
		ctx := context.Background()
		messages, err := client.LoadMessages(ctx, chatID, limit)
		return messagesLoadedMsg{chatID: chatID, messages: messages, err: err, limit: limit, silent: false}
	}
}

func (m Model) refreshActiveMessages() tea.Cmd {
	client := m.client
	chatID := m.state.ActiveChatID
	limit := m.messageLimit(chatID)

	return func() tea.Msg {
		ctx := context.Background()
		messages, err := client.LoadMessages(ctx, chatID, limit)
		return messagesLoadedMsg{
			chatID:         chatID,
			messages:       messages,
			err:            err,
			limit:          limit,
			preserveScroll: true,
			silent:         true,
		}
	}
}

func (m Model) loadMoreMessagesForActiveChat() tea.Cmd {
	client := m.client
	chatID := m.state.ActiveChatID
	oldMessages := m.state.MessagesByChat[chatID]
	currentLimit := m.messageLimit(chatID)
	nextLimit := currentLimit + 80
	if nextLimit > 1000 {
		nextLimit = 1000
	}

	return func() tea.Msg {
		ctx := context.Background()
		messages, err := client.LoadMessages(ctx, chatID, nextLimit)
		return messagesLoadedMsg{
			chatID:        chatID,
			messages:      messages,
			err:           err,
			limit:         nextLimit,
			preserveTop:   true,
			previousCount: len(oldMessages),
			silent:        false,
		}
	}
}

func (m Model) scheduleRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) sendMessage(chatID domains.ChatID, text string, replyToMessageID int64) tea.Cmd {
	client := m.client

	return func() tea.Msg {
		ctx := context.Background()
		message, err := client.SendMessage(ctx, chatID, text, replyToMessageID)
		return messageSentMsg{chatID: chatID, message: message, err: err}
	}
}

func (m *Model) activeChatIndex() int {
	if len(m.state.Chats) == 0 {
		return -1
	}

	for i, chat := range m.state.Chats {
		if chat.ID == m.state.ActiveChatID {
			return i
		}
	}

	return -1
}

func (m *Model) selectRelativeChat(delta int) bool {
	if len(m.state.Chats) == 0 {
		return false
	}

	index := m.activeChatIndex()
	if index == -1 {
		index = 0
	}

	next := index + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.state.Chats) {
		next = len(m.state.Chats) - 1
	}

	if m.state.Chats[next].ID == m.state.ActiveChatID {
		return false
	}

	m.state.ActiveChatID = m.state.Chats[next].ID
	return true
}

func oneLine(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	replacer := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")
	compact := replacer.Replace(trimmed)
	compact = strings.Map(func(r rune) rune {
		// Strip control characters that can break terminal layout.
		if unicode.IsControl(r) {
			return -1
		}

		// Strip emoji and emoji-related joiner/selector runes because some
		// terminal font stacks report inconsistent width for these sequences.
		if isEmojiRune(r) {
			return -1
		}
		return r
	}, compact)
	return strings.Join(strings.Fields(compact), " ")
}

func isEmojiRune(r rune) bool {
	if r == '\u200d' || r == '\ufe0f' || r == '\u20e3' {
		return true
	}

	if r >= 0x1f3fb && r <= 0x1f3ff {
		return true
	}

	if r >= 0x1f1e6 && r <= 0x1f1ff {
		return true
	}

	if r >= 0x1f300 && r <= 0x1faff {
		return true
	}

	return unicode.Is(unicode.So, r)
}

func truncateRunes(value string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}

	if maxLen == 1 {
		return "…"
	}

	return string(runes[:maxLen-1]) + "…"
}

func truncateDisplayWidth(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	clean := oneLine(value)
	if clean == "" {
		return ""
	}

	return runewidth.Truncate(clean, maxWidth, "…")
}

func (m Model) maxMessageScroll() int {
	messages, ok := m.state.MessagesByChat[m.state.ActiveChatID]
	if !ok {
		return 0
	}
	if len(messages) <= 1 {
		return 0
	}

	return len(messages) - 1
}

func (m *Model) scrollMessages(delta int) bool {
	_, ok := m.state.MessagesByChat[m.state.ActiveChatID]
	if !ok {
		return false
	}

	maxScroll := m.maxMessageScroll()
	next := m.messageScroll + delta
	if next < 0 {
		next = 0
	}
	if next > maxScroll {
		next = maxScroll
	}

	if next == m.messageScroll {
		return false
	}

	m.messageScroll = next
	return true
}

func (m *Model) moveMessageSelection(delta int) bool {
	messages, ok := m.state.MessagesByChat[m.state.ActiveChatID]
	if !ok || len(messages) == 0 {
		return false
	}

	current := m.selectedMessageByChat[m.state.ActiveChatID]
	if current < 0 || current >= len(messages) {
		current = len(messages) - 1
	}

	next := current + delta
	if next < 0 {
		next = 0
	}
	if next >= len(messages) {
		next = len(messages) - 1
	}

	if next == current {
		return false
	}

	m.selectedMessageByChat[m.state.ActiveChatID] = next
	m.keepSelectedMessageVisible(len(messages), next)
	return true
}

func (m *Model) keepSelectedMessageVisible(totalMessages int, selectedIndex int) {
	if totalMessages <= 0 {
		m.messageScroll = 0
		return
	}

	visibleCount := m.estimatedVisibleMessageCount()
	if visibleCount < 1 {
		visibleCount = 1
	}

	visibleNewest := totalMessages - 1 - m.messageScroll
	if visibleNewest < 0 {
		visibleNewest = 0
	}
	visibleOldest := visibleNewest - visibleCount + 1
	if visibleOldest < 0 {
		visibleOldest = 0
	}

	if selectedIndex < visibleOldest {
		visibleNewest = selectedIndex + visibleCount - 1
		if visibleNewest > totalMessages-1 {
			visibleNewest = totalMessages - 1
		}
		m.messageScroll = (totalMessages - 1) - visibleNewest
	}

	if selectedIndex > visibleNewest {
		m.messageScroll = (totalMessages - 1) - selectedIndex
	}

	if m.messageScroll < 0 {
		m.messageScroll = 0
	}
	if maxScroll := m.maxMessageScroll(); m.messageScroll > maxScroll {
		m.messageScroll = maxScroll
	}
}

func (m Model) estimatedVisibleMessageCount() int {
	leftWidth := max(30, m.width/3)
	rightWidth := max(40, m.width-leftWidth-8)
	_ = rightWidth

	panelHeight := max(12, m.height-6)
	contentRows := max(1, panelHeight-2)

	messages, loaded := m.state.MessagesByChat[m.state.ActiveChatID]
	hasScrollLine := loaded && len(messages) > 1
	footerRows := 4
	if hasScrollLine {
		footerRows++
	}

	rows := max(1, contentRows-1-footerRows)
	return rows
}

func (m Model) messageLimit(chatID domains.ChatID) int {
	if limit, ok := m.messageLimitByChat[chatID]; ok && limit > 0 {
		return limit
	}

	return 80
}

func (m Model) canLoadOlderMessages() bool {
	if m.state.ActiveChatID == 0 {
		return false
	}

	messages, ok := m.state.MessagesByChat[m.state.ActiveChatID]
	if !ok || len(messages) == 0 {
		return false
	}

	limit := m.messageLimit(m.state.ActiveChatID)
	if limit >= 1000 {
		return false
	}

	// If server returned fewer than requested, there is likely no older page left.
	return len(messages) >= limit
}

func (m Model) buildMessageViewLines(messages []domains.Message, width int, rowBudget int) []string {
	if len(messages) == 0 || rowBudget <= 0 {
		return nil
	}

	end := len(messages) - m.messageScroll
	if end < 0 {
		end = 0
	}
	if end > len(messages) {
		end = len(messages)
	}

	rowsRemaining := rowBudget
	blocks := make([][]string, 0, rowBudget)

	for i := end - 1; i >= 0 && rowsRemaining > 0; i-- {
		selected := m.selectedMessageByChat[m.state.ActiveChatID] == i
		block := renderMessageBlock(messages[i], width, selected)
		if len(block) == 0 {
			continue
		}

		need := len(block)
		if len(blocks) > 0 {
			need++ // spacer line between messages.
		}
		if need > rowsRemaining {
			break
		}

		if len(blocks) > 0 {
			blocks = append(blocks, []string{""})
			rowsRemaining--
		}

		blocks = append(blocks, block)
		rowsRemaining -= len(block)
	}

	result := make([]string, 0, rowBudget)
	for i := len(blocks) - 1; i >= 0; i-- {
		result = append(result, blocks[i]...)
	}

	return result
}

func renderMessageBlock(message domains.Message, width int, selected bool) []string {
	var name string
	var nameStyle lipgloss.Style
	if message.Direction == domains.MessageDirectionOutgoing {
		name = "You"
		nameStyle = outgoingNameStyle
	} else {
		name = oneLine(message.SenderName)
		if name == "" {
			name = "Incoming"
		}
		nameStyle = incomingNameStyle
	}

	body := oneLine(message.Text)
	if body == "" {
		body = "[empty]"
	}

	prefixText := name + ": "
	prefixWidth := lipgloss.Width(prefixText)
	contentWidth := width - prefixWidth
	if contentWidth < 8 {
		contentWidth = width
		prefixText = ""
		prefixWidth = 0
	}

	wrappedBody := runewidth.Wrap(body, contentWidth)
	bodyLines := strings.Split(wrappedBody, "\n")
	for i := range bodyLines {
		bodyLines[i] = truncateDisplayWidth(bodyLines[i], contentWidth)
	}

	line := ""
	if len(bodyLines) > 0 {
		line = prefixText + bodyLines[0]
	}
	line = oneLine(line)
	if line == "" {
		line = name + ":"
	}

	rawLines := make([]string, 0, len(bodyLines))
	rawLines = append(rawLines, line)
	if len(bodyLines) > 1 {
		indent := strings.Repeat(" ", prefixWidth)
		for _, rest := range bodyLines[1:] {
			rawLines = append(rawLines, indent+rest)
		}
	}

	out := make([]string, 0, len(rawLines))
	for i, raw := range rawLines {
		clean := truncateDisplayWidth(raw, width)
		if i == 0 && prefixText != "" {
			clean = colorizePrefix(clean, name+":", nameStyle)
		}
		if selected {
			clean = selectedMessageStyle.Render(clean)
		}
		if message.Direction == domains.MessageDirectionOutgoing {
			out = append(out, lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(clean))
		} else {
			out = append(out, lipgloss.NewStyle().Width(width).Align(lipgloss.Left).Render(clean))
		}
	}

	return out
}

func clampLines(lines []string, maxRows int) []string {
	if maxRows <= 0 {
		return nil
	}

	if len(lines) <= maxRows {
		return lines
	}

	return lines[:maxRows]
}

func colorizePrefix(line string, prefix string, style lipgloss.Style) string {
	if !strings.HasPrefix(line, prefix) {
		return line
	}

	return style.Render(prefix) + line[len(prefix):]
}

func isMouseEscapeKey(msg tea.KeyMsg) bool {
	raw := string(msg.Runes)
	if raw == "" {
		return false
	}

	// xterm mouse tracking escape sequences (SGR and legacy forms) may leak as
	// text in some terminal/multiplexer combos; drop them from input handling.
	if strings.HasPrefix(raw, "\x1b[<") && strings.Contains(raw, ";") {
		if strings.HasSuffix(raw, "M") || strings.HasSuffix(raw, "m") {
			return true
		}
	}
	if strings.HasPrefix(raw, "\x1b[M") {
		return true
	}

	return false
}

func previewReplyText(text string) string {
	one := oneLine(text)
	if one == "" {
		return "[empty]"
	}
	return truncateDisplayWidth(one, 48)
}

func (m *Model) selectedCurrentMessage() (domains.Message, bool) {
	messages, ok := m.state.MessagesByChat[m.state.ActiveChatID]
	if !ok || len(messages) == 0 {
		return domains.Message{}, false
	}

	idx := m.selectedMessageByChat[m.state.ActiveChatID]
	if idx < 0 || idx >= len(messages) {
		return domains.Message{}, false
	}

	return messages[idx], true
}

func (m *Model) handleMouseClick(mouse tea.MouseMsg) tea.Cmd {
	leftWidth := max(30, m.width/3)
	panelHeight := max(12, m.height-6)
	contentRows := max(1, panelHeight-2)

	panelTopY := 2
	panelContentTopY := panelTopY + 1
	panelLeftX := 2

	if mouse.Y < panelTopY || mouse.Y >= panelTopY+panelHeight {
		return nil
	}

	contentRow := mouse.Y - panelContentTopY
	if contentRow < 0 {
		contentRow = 0
	}
	if contentRow >= contentRows {
		contentRow = contentRows - 1
	}

	if mouse.X >= panelLeftX && mouse.X < panelLeftX+leftWidth {
		chatIndex, ok := m.chatIndexAtContentRow(contentRow, contentRows)
		if !ok {
			return nil
		}
		if chatIndex >= 0 && chatIndex < len(m.state.Chats) {
			m.state.ActiveChatID = m.state.Chats[chatIndex].ID
			m.messageView = false
			m.messageScroll = 0
			m.composeInput.Blur()
			m.state.Status = "Chat selected. Press Enter/Right to open"
		}
		return nil
	}

	rightStartX := panelLeftX + leftWidth
	if mouse.X < rightStartX {
		return nil
	}

	if !m.messageView {
		if m.state.ActiveChatID != 0 && m.client != nil {
			m.state.Status = "Loading messages"
			m.state.Error = nil
			m.composeInput.Focus()
			return m.loadMessagesForActiveChat()
		}
		return nil
	}

	msgIndex, ok := m.messageIndexAtContentRow(contentRow, contentRows)
	if ok {
		m.selectedMessageByChat[m.state.ActiveChatID] = msgIndex
		m.state.Status = "Message selected (press Ctrl+r to reply)"
	}

	return nil
}

func (m Model) chatIndexAtContentRow(contentRow int, maxRows int) (int, bool) {
	if len(m.state.Chats) == 0 {
		return 0, false
	}

	currentIndex := m.activeChatIndex()
	if currentIndex == -1 {
		currentIndex = 0
	}

	visibleRows := max(1, maxRows-3)
	start := currentIndex - visibleRows/2
	if start < 0 {
		start = 0
	}
	if start+visibleRows > len(m.state.Chats) {
		start = len(m.state.Chats) - visibleRows
		if start < 0 {
			start = 0
		}
	}
	end := start + visibleRows
	if end > len(m.state.Chats) {
		end = len(m.state.Chats)
	}

	lineNo := 0
	lineNo++ // header
	if start > 0 {
		lineNo++ // leading ellipsis row
	}

	for i := start; i < end; i++ {
		if contentRow == lineNo {
			return i, true
		}
		lineNo++
	}

	return 0, false
}

func (m Model) messageIndexAtContentRow(contentRow int, maxRows int) (int, bool) {
	messages, loaded := m.state.MessagesByChat[m.state.ActiveChatID]
	if !loaded || len(messages) == 0 {
		return 0, false
	}

	rightWidth := max(40, m.width-max(30, m.width/3)-8)
	messageWidth := max(20, rightWidth-6)
	hasScrollLine := len(messages) > 1
	footerRows := 4
	if hasScrollLine {
		footerRows++
	}
	messageRows := max(0, maxRows-1-footerRows)
	if messageRows <= 0 {
		return 0, false
	}

	lineNo := 1 // header row

	end := len(messages) - m.messageScroll
	if end < 0 {
		end = 0
	}
	if end > len(messages) {
		end = len(messages)
	}

	rowsRemaining := messageRows
	type blockInfo struct {
		idx   int
		lines int
	}
	blocks := make([]blockInfo, 0, messageRows)

	for i := end - 1; i >= 0 && rowsRemaining > 0; i-- {
		count := len(renderMessageBlock(messages[i], messageWidth, false))
		if count == 0 {
			continue
		}
		need := count
		if len(blocks) > 0 {
			need++
		}
		if need > rowsRemaining {
			break
		}

		if len(blocks) > 0 {
			rowsRemaining--
		}
		blocks = append(blocks, blockInfo{idx: i, lines: count})
		rowsRemaining -= count
	}

	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		for row := 0; row < b.lines; row++ {
			if contentRow == lineNo {
				return b.idx, true
			}
			lineNo++
		}
		if i > 0 {
			lineNo++ // spacer
		}
	}

	return 0, false
}
