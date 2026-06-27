package domains

type EventType string

const (
	EventTypeChatsLoaded     EventType = "chats_loaded"
	EventTypeMessagesLoaded  EventType = "messages_loaded"
	EventTypeMessageReceived EventType = "message_received"
	EventTypeMessageSent     EventType = "message_sent"
	EventTypeStatusChanged   EventType = "status_changed"
	EventTypeError           EventType = "error"
)

type AppEvent struct {
	Type     EventType
	ChatID   ChatID
	Chats    []ChatSummary
	Messages []Message
	Message  *Message
	Status   string
	Err      error
}
