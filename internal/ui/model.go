package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tg-tui/internal/app"
	"tg-tui/internal/domains"
)

var (
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type Model struct {
	state        app.State
	client       app.TelegramClient
	width        int
	height       int
	authInput    textinput.Model
	composeInput textinput.Model
}

type authResultMsg struct {
	state   app.AuthState
	session domains.AccountSession
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

	return Model{
		state:        state,
		client:       client,
		authInput:    authInput,
		composeInput: composeInput,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		return m, nil
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
		if typed.session.Authorized {
			m.state.Status = "Authorized. Chat sync is next."
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
		return m, nil
	case tea.KeyMsg:
		switch typed.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
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
		case "r":
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

	return lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle.Width(leftWidth).Height(max(12, m.height-6)).Render(m.renderChats()),
		panelStyle.Width(rightWidth).Height(max(12, m.height-6)).Render(m.renderMessages()),
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
	lines = append(lines, mutedStyle.Render("Telegram credentials can come from TG_TUI_API_ID/TG_TUI_API_HASH or shared defaults."))
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

func (m Model) renderChats() string {
	lines := []string{headerStyle.Render("Chats")}
	if len(m.state.Chats) == 0 {
		lines = append(lines, mutedStyle.Render("No chats loaded yet."))
		return strings.Join(lines, "\n")
	}

	for _, chat := range m.state.Chats {
		prefix := "  "
		if chat.ID == m.state.ActiveChatID {
			prefix = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, chat.Title))
		if chat.LastMessageText != "" {
			lines = append(lines, mutedStyle.Render("   "+chat.LastMessageText))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderMessages() string {
	lines := []string{headerStyle.Render("Messages")}
	messages := m.state.ActiveMessages()
	if len(messages) == 0 {
		lines = append(lines, mutedStyle.Render("No messages loaded yet."))
	} else {
		for _, message := range messages {
			prefix := message.SenderName
			if prefix == "" {
				prefix = string(message.Direction)
			}
			lines = append(lines, fmt.Sprintf("%s: %s", prefix, message.Text))
		}
	}

	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Compose"))
	lines = append(lines, m.composeInput.View())

	return strings.Join(lines, "\n")
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
