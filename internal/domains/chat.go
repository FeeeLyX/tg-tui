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

type ChatFolder struct {
	ID    int
	Title string
}

type ChatSummary struct {
	ID              ChatID
	Title           string
	LastMessageText string
	LastMessageAt   time.Time
	UnreadCount     int
	Pinned          bool
	FolderID        int
	FolderIDs       []int
	FolderTitle     string
	IsOnline        bool
}

type Message struct {
	ID                int64
	ChatID            ChatID
	SenderName        string
	Text              string
	HasImage          bool
	ImagePreviewASCII string
	ImageFullASCII    string
	ReplyToMessageID  int64
	ReplyToSenderName string
	ReplyToText       string
	Direction         MessageDirection
	SentAt            time.Time
	DeliveredAt       *time.Time
	Pending           bool
	Failed            bool
}
