package domains

import "time"

type ChatID int64

type MessageDirection string

const (
	MessageDirectionIncoming MessageDirection = "incoming"
	MessageDirectionOutgoing MessageDirection = "outgoing"
)

type AccountSession struct {
	Authorized bool
	Phone      string
	UpdatedAt  time.Time
}

type ChatSummary struct {
	ID              ChatID
	Title           string
	LastMessageText string
	LastMessageAt   time.Time
	UnreadCount     int
	IsOnline        bool
}

type Message struct {
	ID                int64
	ChatID            ChatID
	SenderName        string
	Text              string
	ReplyToMessageID  int64
	ReplyToSenderName string
	ReplyToText       string
	Direction         MessageDirection
	SentAt            time.Time
	DeliveredAt       *time.Time
	Pending           bool
	Failed            bool
}
