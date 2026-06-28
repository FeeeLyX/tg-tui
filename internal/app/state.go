package app

import "github.com/FeeeLyX/tg-tui/internal/domains"

type State struct {
	AuthState         AuthState
	Session           domains.AccountSession
	CredentialSummary string
	CredentialNotice  string
	Folders           []domains.ChatFolder
	ActiveFolderID    int
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
		Folders:        []domains.ChatFolder{{ID: 0, Title: "All"}},
		ActiveFolderID: 0,
		MessagesByChat: map[domains.ChatID][]domains.Message{},
		Status:         "Bootstrapping",
	}
}

func (s State) ActiveMessages() []domains.Message {
	return s.MessagesByChat[s.ActiveChatID]
}

func ApplyCachedChats(state State, chats []domains.ChatSummary) State {
	state.Chats = chats
	if state.ActiveChatID == 0 && len(chats) > 0 {
		state.ActiveChatID = chats[0].ID
	}
	return state
}

func ApplyCachedMessages(state State, chatID domains.ChatID, messages []domains.Message) State {
	state.MessagesByChat[chatID] = messages
	return state
}
