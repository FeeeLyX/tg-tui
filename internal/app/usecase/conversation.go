package usecase

import "github.com/FeeeLyX/tg-tui/internal/domains"

type Conversation struct{}

type SelectionRefreshInput struct {
	PreviousMessages     []domains.Message
	NewMessages          []domains.Message
	CurrentSelectedIndex int
	HasSelection         bool
	PreserveTop          bool
	PreserveScroll       bool
	PreviousCount        int
}

func NewConversation() Conversation {
	return Conversation{}
}

func (Conversation) MoveSelection(total, current, delta int) (int, bool) {
	if total <= 0 {
		return 0, false
	}

	if current < 0 || current >= total {
		current = total - 1
	}

	next := current + delta
	if next < 0 {
		next = 0
	}
	if next >= total {
		next = total - 1
	}

	return next, next != current
}

func (Conversation) SelectedMessage(messages []domains.Message, selectedIndex int) (domains.Message, bool) {
	if len(messages) == 0 || selectedIndex < 0 || selectedIndex >= len(messages) {
		return domains.Message{}, false
	}
	return messages[selectedIndex], true
}

func (Conversation) ResolveReplyTarget(messages []domains.Message, replyID int64) (domains.Message, bool) {
	if len(messages) == 0 || replyID <= 0 {
		return domains.Message{}, false
	}

	for i := range messages {
		if messages[i].ID == replyID {
			return messages[i], true
		}
	}

	return domains.Message{}, false
}

func (Conversation) SetReplyTarget(replyMap map[domains.ChatID]int64, chatID domains.ChatID, messageID int64) {
	if replyMap == nil || chatID == 0 || messageID <= 0 {
		return
	}
	replyMap[chatID] = messageID
}

func (Conversation) ClearReplyTarget(replyMap map[domains.ChatID]int64, chatID domains.ChatID) {
	if replyMap == nil || chatID == 0 {
		return
	}
	delete(replyMap, chatID)
}

func (Conversation) ReconcileSelection(input SelectionRefreshInput) int {
	newCount := len(input.NewMessages)
	if newCount == 0 {
		return 0
	}

	if input.PreserveTop && input.HasSelection {
		delta := len(input.NewMessages) - input.PreviousCount
		next := input.CurrentSelectedIndex
		if delta > 0 {
			next += delta
		}
		if next < 0 {
			next = 0
		}
		if next >= newCount {
			next = newCount - 1
		}
		return next
	}

	if input.PreserveScroll && input.HasSelection {
		var previousSelectedID int64
		if input.CurrentSelectedIndex >= 0 && input.CurrentSelectedIndex < len(input.PreviousMessages) {
			previousSelectedID = input.PreviousMessages[input.CurrentSelectedIndex].ID
		}

		if previousSelectedID != 0 {
			for i := range input.NewMessages {
				if input.NewMessages[i].ID == previousSelectedID {
					return i
				}
			}
		}

		next := input.CurrentSelectedIndex
		if next < 0 {
			next = 0
		}
		if next >= newCount {
			next = newCount - 1
		}
		return next
	}

	return newCount - 1
}
