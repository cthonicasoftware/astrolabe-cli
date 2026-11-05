package root

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

// startTCPSession starts a TUI capture session against ln with reconnect on.
func startTCPSession(t *testing.T, ln net.Listener, cacheRoot string) tui.CaptureSession {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	port := ln.Addr().(*net.TCPAddr).Port
	sess, err := newCaptureAdapter(nil, cacheRoot).Start(t.Context(), tui.CaptureConfig{
		SourceType: tui.SourceTypeTCP,
		TCPHost:    "127.0.0.1",
		TCPPort:    strconv.Itoa(port),
		Reconnect:  true,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return sess
}

// nextChunk returns the next feed chunk, failing after a timeout.
func nextChunk(t *testing.T, feed <-chan tui.FeedChunk) tui.FeedChunk {
	t.Helper()
	select {
	case c, ok := <-feed:
		if !ok {
			t.Fatal("feed closed early")
		}
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for feed")
	}
	return tui.FeedChunk{}
}

// nextLink skips data chunks until a link event arrives.
func nextLink(t *testing.T, feed <-chan tui.FeedChunk) *core.LinkEvent {
	t.Helper()
	for {
		if c := nextChunk(t, feed); c.Link != nil {
			return c.Link
		}
	}
}

func TestCaptureSession_LinkEventsReachFeed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	release := make(chan struct{})
	defer close(release)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = c.Write([]byte("one\n"))
		time.Sleep(50 * time.Millisecond)
		_ = c.Close()
		c, err = ln.Accept()
		if err != nil {
			return
		}
		_, _ = c.Write([]byte("two\n"))
		<-release
		_ = c.Close()
	}()

	cacheRoot := t.TempDir()
	sess := startTCPSession(t, ln, cacheRoot)
	feed := sess.Feed()

	var order []string
	for len(order) < 4 {
		c := nextChunk(t, feed)
		switch {
		case c.Link != nil:
			if c.At.IsZero() {
				t.Error("link chunk has no time")
			}
			order = append(order, "link_"+string(c.Link.State))
		case strings.Contains(c.Data, "one"):
			order = append(order, "one")
		case strings.Contains(c.Data, "two"):
			order = append(order, "two")
		}
	}
	want := []string{"one", "link_down", "link_up", "two"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("feed order = %v, want %v", order, want)
	}

	sess.RequestSave()
	res := sess.Stop()
	if res.Err != nil || !res.Saved {
		t.Fatalf("Stop = %+v, want saved", res)
	}
	data, err := os.ReadFile(filepath.Join(cacheRoot, res.RunID, "data.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{capture.RecordTypeLinkDown, capture.RecordTypeLinkUp} {
		if !strings.Contains(string(data), `"type":"`+typ+`"`) {
			t.Errorf("saved data.jsonl has no %s record", typ)
		}
	}
	doc, err := storage.LoadManifest(filepath.Join(cacheRoot, res.RunID, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Manifest.Attributes[capture.AttrReconnects]; got != "1" {
		t.Errorf("saved manifest reconnects = %q, want \"1\"", got)
	}
}

func TestCaptureSession_SendWhileDownRecordsNothing(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		// Stop listening so reconnect attempts are refused and the link
		// stays down.
		_ = ln.Close()
		time.Sleep(50 * time.Millisecond)
		_ = c.Close()
	}()

	cacheRoot := t.TempDir()
	sess := startTCPSession(t, ln, cacheRoot)
	if ev := nextLink(t, sess.Feed()); ev.State != core.LinkDown {
		t.Fatalf("first link event = %q, want down", ev.State)
	}

	if _, err := sess.Send("AT", sources.LineEndingLF); !errors.Is(err, sources.ErrLinkDown) {
		t.Fatalf("Send while down: err = %v, want ErrLinkDown", err)
	}

	sess.RequestSave()
	res := sess.Stop()
	if res.Err != nil || !res.Saved {
		t.Fatalf("Stop = %+v, want saved", res)
	}
	f, err := os.Open(filepath.Join(cacheRoot, res.RunID, "data.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sawDown := false
	for sc.Scan() {
		var rec struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("bad record %q: %v", sc.Text(), err)
		}
		if rec.Type == capture.RecordTypeTX {
			t.Errorf("run has a tx record for a send that never reached the device: %s", sc.Text())
		}
		if rec.Type == capture.RecordTypeLinkDown {
			sawDown = true
		}
	}
	if !sawDown {
		t.Error("saved data.jsonl has no link_down record")
	}

	doc, err := storage.LoadManifest(filepath.Join(cacheRoot, res.RunID, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Manifest.Attributes[capture.AttrReconnects]; got != "0" {
		t.Errorf("saved manifest reconnects = %q, want \"0\"", got)
	}
}
