package main

import "testing"

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
