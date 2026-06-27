package usecase

import (
	"testing"

	"tg-tui/internal/domains"
)

func TestListNavigationSelectRelative(t *testing.T) {
	nav := NewListNavigation()
	chats := []domains.ChatSummary{{ID: 1}, {ID: 2}, {ID: 3}}

	nextID, changed := nav.SelectRelative(chats, 2, 1)
	if !changed || nextID != 3 {
		t.Fatalf("expected move to next chat id=3, got id=%d changed=%t", nextID, changed)
	}

	nextID, changed = nav.SelectRelative(chats, 3, 1)
	if changed {
		t.Fatalf("expected no change at upper boundary")
	}
	if nextID != 3 {
		t.Fatalf("expected boundary id=3, got %d", nextID)
	}
}

func TestListNavigationVisibleWindow(t *testing.T) {
	nav := NewListNavigation()

	start, end := nav.VisibleWindow(10, 5, 12)
	if start >= end {
		t.Fatalf("expected non-empty window, got start=%d end=%d", start, end)
	}
	if start < 0 || end > 10 {
		t.Fatalf("window out of bounds: start=%d end=%d", start, end)
	}
}

func TestListNavigationChatIndexAtContentRow(t *testing.T) {
	nav := NewListNavigation()

	idx, ok := nav.ChatIndexAtContentRow(1, 12, 6, 0)
	if !ok {
		t.Fatalf("expected first chat row to be selectable")
	}
	if idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
}
