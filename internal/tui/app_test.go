package tui

import (
	"testing"
	"time"
)

// Only one pullLine may be in flight at a time. If tick or resume also start a
// reader, several goroutines race on the feed channel and chunks reach the
// LineBuffer out of order (e.g. a line's first letter lands on the next line).

func TestAppTickDoesNotPullFromFeed(t *testing.T) {
	feed := make(chan FeedChunk, 1)
	feed <- FeedChunk{Data: "T", At: testAt}
	app := NewApp("test", feed)

	_, cmd := app.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("tick should re-arm the ticker")
	}
	if _, ok := cmd().(tickMsg); !ok {
		t.Fatal("tick returned a batched command; it must only re-arm the ticker, not pull from the feed")
	}
	if len(feed) != 1 {
		t.Fatal("tick consumed data from the feed")
	}
}

func TestAppResumeDoesNotStartSecondReader(t *testing.T) {
	app := NewApp("test", make(chan FeedChunk))

	if _, cmd := app.togglePause(); cmd != nil {
		t.Fatal("pause returned a command")
	}
	if _, cmd := app.togglePause(); cmd != nil {
		t.Fatal("resume returned a command; handleLineData already keeps one reader running")
	}
}

func TestAppFeedChunksStayInOrder(t *testing.T) {
	chunks := []string{"T", "emp: 22\r\n", "H", "um: 40\r\n"}
	feed := make(chan FeedChunk, len(chunks))
	for _, c := range chunks {
		feed <- FeedChunk{Data: c, At: testAt}
	}
	close(feed)

	app := NewApp("test", feed)
	var model Screen = app

	// Drive the single reader chain the way Bubble Tea would.
	cmd := app.pullLine()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		model, cmd = model.Update(msg)
	}

	got := app.lineBuffer.GetDisplayLines()
	want := []string{"Temp: 22", "Hum: 40"}
	if len(got) != len(want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines = %q, want %q", got, want)
		}
	}
}
