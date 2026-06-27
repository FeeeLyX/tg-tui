package usecase

import "tg-tui/internal/domains"

type ListNavigation struct{}

func NewListNavigation() ListNavigation {
	return ListNavigation{}
}

func (ListNavigation) ActiveIndex(chats []domains.ChatSummary, activeChatID domains.ChatID) int {
	if len(chats) == 0 {
		return -1
	}

	for i := range chats {
		if chats[i].ID == activeChatID {
			return i
		}
	}

	return -1
}

func (n ListNavigation) SelectRelative(chats []domains.ChatSummary, activeChatID domains.ChatID, delta int) (domains.ChatID, bool) {
	if len(chats) == 0 {
		return 0, false
	}

	index := n.ActiveIndex(chats, activeChatID)
	if index == -1 {
		index = 0
	}

	next := index + delta
	if next < 0 {
		next = 0
	}
	if next >= len(chats) {
		next = len(chats) - 1
	}

	nextID := chats[next].ID
	if nextID == activeChatID {
		return nextID, false
	}

	return nextID, true
}

func (ListNavigation) VisibleWindow(totalChats int, currentIndex int, maxRows int) (int, int) {
	if totalChats <= 0 {
		return 0, 0
	}
	if currentIndex < 0 {
		currentIndex = 0
	}
	if currentIndex >= totalChats {
		currentIndex = totalChats - 1
	}

	visibleChats := maxInt(1, (maxRows-2)/2)
	start := currentIndex - visibleChats/2
	if start < 0 {
		start = 0
	}
	if start+visibleChats > totalChats {
		start = totalChats - visibleChats
		if start < 0 {
			start = 0
		}
	}
	end := start + visibleChats
	if end > totalChats {
		end = totalChats
	}

	return start, end
}

func (n ListNavigation) ChatIndexAtContentRow(contentRow int, maxRows int, totalChats int, currentIndex int) (int, bool) {
	if totalChats == 0 {
		return 0, false
	}

	start, end := n.VisibleWindow(totalChats, currentIndex, maxRows)

	lineNo := 0
	lineNo++ // header
	if start > 0 {
		lineNo++ // leading ellipsis
	}

	for i := start; i < end; i++ {
		if contentRow == lineNo {
			return i, true
		}
		lineNo++
		if i < end-1 {
			lineNo++ // separator
		}
	}

	return 0, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
