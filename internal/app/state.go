package app

import "tg-tui/internal/domains"

type State struct {
	AuthState         AuthState
	Session           domains.AccountSession
	CredentialSummary string
	CredentialNotice  string
	Chats             []domains.ChatSummary
	MessagesByChat    map[domains.ChatID][]domains.Message
	ActiveChatID      domains.ChatID
	Status            string
	Error             error
	Ready             bool
}

func NewState() State {
	return State{
		AuthState:      AuthState{Step: AuthStepPhone},
		MessagesByChat: map[domains.ChatID][]domains.Message{},
		Status:         "Bootstrapping",
	}
}

func (s State) ActiveMessages() []domains.Message {
	return s.MessagesByChat[s.ActiveChatID]
}
