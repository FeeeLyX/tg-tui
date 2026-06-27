package usecase

import (
	"testing"

	"tg-tui/internal/domains"
)

func TestConversationMoveSelection_ClampBounds(t *testing.T) {
	uc := NewConversation()

	next, changed := uc.MoveSelection(3, 2, 1)
	if changed {
		t.Fatalf("expected no change at upper bound")
	}
	if next != 2 {
		t.Fatalf("expected clamped upper index, got %d", next)
	}

	next, changed = uc.MoveSelection(3, 0, -1)
	if changed {
		t.Fatalf("expected no change at lower bound")
	}
	if next != 0 {
		t.Fatalf("expected clamped lower index, got %d", next)
	}
}

func TestConversationSelectedMessage_InvalidIndex(t *testing.T) {
	uc := NewConversation()
	messages := []domains.Message{{ID: 1}, {ID: 2}}

	_, ok := uc.SelectedMessage(messages, -1)
	if ok {
		t.Fatalf("expected invalid selection for negative index")
	}

	_, ok = uc.SelectedMessage(messages, 2)
	if ok {
		t.Fatalf("expected invalid selection for out-of-range index")
	}
}

func TestConversationReconcileSelection_PreserveScrollByID(t *testing.T) {
	uc := NewConversation()
	prev := []domains.Message{{ID: 10}, {ID: 20}, {ID: 30}}
	next := []domains.Message{{ID: 5}, {ID: 10}, {ID: 20}, {ID: 30}}

	idx := uc.ReconcileSelection(SelectionRefreshInput{
		PreviousMessages:     prev,
		NewMessages:          next,
		CurrentSelectedIndex: 1,
		HasSelection:         true,
		PreserveScroll:       true,
	})

	if idx != 2 {
		t.Fatalf("expected selection to follow message id, got %d", idx)
	}
}

func TestConversationReconcileSelection_PreserveTopDelta(t *testing.T) {
	uc := NewConversation()
	prev := []domains.Message{{ID: 10}, {ID: 20}}
	next := []domains.Message{{ID: 1}, {ID: 2}, {ID: 10}, {ID: 20}}

	idx := uc.ReconcileSelection(SelectionRefreshInput{
		PreviousMessages:     prev,
		NewMessages:          next,
		CurrentSelectedIndex: 1,
		HasSelection:         true,
		PreserveTop:          true,
		PreviousCount:        2,
	})

	if idx != 3 {
		t.Fatalf("expected preserve-top delta to shift selection, got %d", idx)
	}
}

func TestConversationReplyTargetOperations(t *testing.T) {
	uc := NewConversation()
	replies := map[domains.ChatID]int64{}
	chatID := domains.ChatID(99)

	uc.SetReplyTarget(replies, chatID, 123)
	if replies[chatID] != 123 {
		t.Fatalf("expected reply target to be set")
	}

	messages := []domains.Message{{ID: 122}, {ID: 123, Text: "reply"}}
	msg, ok := uc.ResolveReplyTarget(messages, replies[chatID])
	if !ok || msg.ID != 123 {
		t.Fatalf("expected reply target message to resolve")
	}

	uc.ClearReplyTarget(replies, chatID)
	if _, ok := replies[chatID]; ok {
		t.Fatalf("expected reply target to be cleared")
	}
}
