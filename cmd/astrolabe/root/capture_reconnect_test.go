package root

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/spf13/cobra"
)

func TestBuildComponents_WrapsWhenReconnectSet(t *testing.T) {
	svc := newCaptureService(nil, t.TempDir())
	tcp := sources.TCPConfig{Host: "127.0.0.1", Port: 9100}

	c, err := svc.BuildComponents(CaptureRequest{Source: tcp, Reconnect: &sources.ReconnectPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Source.(*sources.Reconnecting); !ok {
		t.Fatalf("source = %T, want *sources.Reconnecting", c.Source)
	}

	c, err = svc.BuildComponents(CaptureRequest{Source: tcp})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Source.(*sources.Reconnecting); ok {
		t.Fatal("source wrapped without Reconnect policy")
	}

	path := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(path, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = svc.BuildComponents(CaptureRequest{
		Source:    sources.FileConfig{Path: path},
		Reconnect: &sources.ReconnectPolicy{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Source.(*sources.Reconnecting); ok {
		t.Fatal("file source must not be wrapped")
	}
}

func TestReconnectPolicyFromFlags(t *testing.T) {
	if p := reconnectPolicyFromFlags(false, time.Minute); p != nil {
		t.Fatalf("disabled policy = %+v, want nil", p)
	}
	p := reconnectPolicyFromFlags(true, 10*time.Minute)
	if p == nil || p.Timeout != 10*time.Minute || p.Initial != 0 || p.Max != 0 {
		t.Fatalf("policy = %+v", p)
	}
}

func TestCaptureCmds_ReconnectFlags(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmds := map[string]*cobra.Command{
		"serial": newCaptureSerialCmd(meta),
		"tcp":    newCaptureTCPCmd(meta),
	}
	for name, cmd := range cmds {
		rc := cmd.Flags().Lookup("reconnect")
		rt := cmd.Flags().Lookup("reconnect-timeout")
		if rc == nil || rc.DefValue != "false" {
			t.Errorf("%s: --reconnect missing or wrong default", name)
		}
		if rt == nil || rt.DefValue != "0s" {
			t.Errorf("%s: --reconnect-timeout missing or wrong default", name)
		}
		if err := cmd.ParseFlags([]string{"--reconnect", "--reconnect-timeout", "10m"}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got, _ := cmd.Flags().GetDuration("reconnect-timeout"); got != 10*time.Minute {
			t.Errorf("%s: timeout = %v", name, got)
		}
	}
}

func TestCaptureService_ReconnectsAcrossTCPServerRestart(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
		time.Sleep(300 * time.Millisecond)
		cancel()
		_ = c.Close()
	}()

	root := t.TempDir()
	svc := newCaptureService(nil, root)
	res, err := svc.Run(ctx, CaptureRequest{
		Source:    sources.TCPConfig{Host: "127.0.0.1", Port: addr.Port, ConnectTimeout: time.Second, BufferSize: 4096},
		Reconnect: &sources.ReconnectPolicy{Initial: 5 * time.Millisecond, Max: 20 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Run.Manifest.Attributes["reconnects"]; got != "1" {
		t.Errorf("reconnects = %q, want 1", got)
	}
	f, err := os.Open(filepath.Join(root, res.Run.ID, "data.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	idx := func(sub string) int {
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i
			}
		}
		return -1
	}
	one, down, up, two := idx("one"), idx("link_down"), idx("link_up"), idx("two")
	if one < 0 || down < 0 || up < 0 || two < 0 || !(one < down && down < up && up < two) {
		t.Fatalf("bad order one=%d down=%d up=%d two=%d\n%s", one, down, up, two, strings.Join(lines, "\n"))
	}
}

func TestCaptureService_WarnsWhenReconnectTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_ = ln.Close() // refuse reconnects
		_, _ = c.Write([]byte("one\n"))
		time.Sleep(50 * time.Millisecond)
		_ = c.Close()
	}()

	var buf bytes.Buffer
	out := cliout.NewPrinter(&buf, false)
	svc := newCaptureService(out, t.TempDir())
	policy := reconnectPolicyWithOutput(out, true, 200*time.Millisecond)
	policy.Initial, policy.Max = 5*time.Millisecond, 20*time.Millisecond
	res, err := svc.Run(context.Background(), CaptureRequest{
		Source:    sources.TCPConfig{Host: "127.0.0.1", Port: addr.Port, ConnectTimeout: 100 * time.Millisecond, BufferSize: 4096},
		Reconnect: policy,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Interrupted {
		t.Fatal("run reported interrupted; want a timeout end")
	}
	if got := buf.String(); !strings.Contains(got, "Reconnect timed out after 200ms; ending capture") {
		t.Errorf("output = %q, want reconnect timeout warning", got)
	}
}
