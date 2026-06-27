package usecase

type MessageView struct{}

func NewMessageView() MessageView {
	return MessageView{}
}

func (MessageView) EstimatedVisibleCount(contentRows int, hasScrollLine bool) int {
	rows := maxInt(1, contentRows-1)
	if hasScrollLine {
		rows = maxInt(1, rows-1)
	}
	return rows
}

func (MessageView) KeepSelectedVisible(totalMessages int, selectedIndex int, currentScroll int, visibleCount int) int {
	if totalMessages <= 0 {
		return 0
	}
	if visibleCount < 1 {
		visibleCount = 1
	}
	if selectedIndex < 0 {
		selectedIndex = 0
	}
	if selectedIndex >= totalMessages {
		selectedIndex = totalMessages - 1
	}

	scroll := currentScroll
	if scroll < 0 {
		scroll = 0
	}
	maxScroll := totalMessages - 1
	if scroll > maxScroll {
		scroll = maxScroll
	}

	visibleNewest := totalMessages - 1 - scroll
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
		scroll = (totalMessages - 1) - visibleNewest
	}

	if selectedIndex > visibleNewest {
		scroll = (totalMessages - 1) - selectedIndex
	}

	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	return scroll
}
