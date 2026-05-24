package main

import (
	"testing"
	"time"
)

func TestAutoTarget(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "english to chinese", text: "hello world", want: "zh-CN"},
		{name: "chinese to english", text: "你好", want: "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autoTarget(tt.text); got != tt.want {
				t.Fatalf("autoTarget(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseGoogleResponse(t *testing.T) {
	body := []byte(`[[["你好","hello",null,null,10]],null,"en",null,null,null,null,[]]`)

	got, source, err := parseGoogleResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if got != "你好" {
		t.Fatalf("translation = %q, want %q", got, "你好")
	}
	if source != "en" {
		t.Fatalf("source = %q, want %q", source, "en")
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := map[string]string{
		"zh":      "zh-CN",
		"zh_cn":   "zh-CN",
		"zh-tw":   "zh-TW",
		"english": "en",
		"ja":      "ja",
	}

	for input, want := range tests {
		if got := normalizeLang(input); got != want {
			t.Fatalf("normalizeLang(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestShouldWaitForStableQuery(t *testing.T) {
	cfg := config{
		CacheDir: t.TempDir(),
		Debounce: 700 * time.Millisecond,
	}
	now := time.Unix(100, 0)

	if wait, ok := shouldWaitForStableQuery(cfg, "hello", now); !ok || wait != cfg.Debounce {
		t.Fatalf("first query wait = %s, %v; want %s, true", wait, ok, cfg.Debounce)
	}
	if wait, ok := shouldWaitForStableQuery(cfg, "hello", now.Add(300*time.Millisecond)); !ok || wait != 400*time.Millisecond {
		t.Fatalf("same query before debounce wait = %s, %v; want 400ms, true", wait, ok)
	}
	if wait, ok := shouldWaitForStableQuery(cfg, "hello", now.Add(800*time.Millisecond)); ok || wait != 0 {
		t.Fatalf("same query after debounce wait = %s, %v; want 0, false", wait, ok)
	}
	if wait, ok := shouldWaitForStableQuery(cfg, "hello!", now.Add(900*time.Millisecond)); !ok || wait != cfg.Debounce {
		t.Fatalf("changed query wait = %s, %v; want %s, true", wait, ok, cfg.Debounce)
	}
}

func TestRerunSeconds(t *testing.T) {
	tests := []struct {
		wait time.Duration
		want float64
	}{
		{wait: 50 * time.Millisecond, want: 0.1},
		{wait: 750 * time.Millisecond, want: 0.75},
		{wait: 8 * time.Second, want: 5.0},
	}

	for _, tt := range tests {
		if got := rerunSeconds(tt.wait); got != tt.want {
			t.Fatalf("rerunSeconds(%s) = %v, want %v", tt.wait, got, tt.want)
		}
	}
}
