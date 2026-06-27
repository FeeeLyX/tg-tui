package usecase

import "testing"

func TestMessageViewEstimatedVisibleCount(t *testing.T) {
	mv := NewMessageView()

	rows := mv.EstimatedVisibleCount(10, false)
	if rows != 9 {
		t.Fatalf("expected 9 visible rows without scroll line, got %d", rows)
	}

	rows = mv.EstimatedVisibleCount(10, true)
	if rows != 8 {
		t.Fatalf("expected 8 visible rows with scroll line, got %d", rows)
	}
}

func TestMessageViewKeepSelectedVisible(t *testing.T) {
	mv := NewMessageView()

	scroll := mv.KeepSelectedVisible(20, 5, 0, 5)
	if scroll <= 0 {
		t.Fatalf("expected scroll adjustment to keep older selection visible, got %d", scroll)
	}

	scroll = mv.KeepSelectedVisible(20, 19, 0, 5)
	if scroll != 0 {
		t.Fatalf("expected no scroll change for newest visible selection, got %d", scroll)
	}
}
