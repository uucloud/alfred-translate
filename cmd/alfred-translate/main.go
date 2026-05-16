package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	defaultTimeout = 8 * time.Second
	cacheTTL       = 7 * 24 * time.Hour
)

type config struct {
	Provider   string
	SourceLang string
	TargetLang string
	Timeout    time.Duration
	CacheDir   string
}

type translation struct {
	Text       string `json:"text"`
	Provider   string `json:"provider"`
	SourceLang string `json:"source_lang,omitempty"`
	TargetLang string `json:"target_lang"`
}

type alfredResponse struct {
	Items []alfredItem `json:"items"`
}

type alfredItem struct {
	UID      string               `json:"uid,omitempty"`
	Title    string               `json:"title"`
	Subtitle string               `json:"subtitle,omitempty"`
	Arg      string               `json:"arg,omitempty"`
	Valid    *bool                `json:"valid,omitempty"`
	Text     *alfredText          `json:"text,omitempty"`
	Mods     map[string]alfredMod `json:"mods,omitempty"`
}

type alfredText struct {
	Copy      string `json:"copy,omitempty"`
	LargeType string `json:"largetype,omitempty"`
}

type alfredMod struct {
	Arg      string `json:"arg,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		writeUsage(os.Stderr)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "filter":
		err = runFilter(os.Args[2:])
	case "translate":
		err = runTranslate(os.Args[2:])
	default:
		writeUsage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runFilter(args []string) error {
	cfg, query, err := parseFlags("filter", args)
	if err != nil {
		writeAlfred(alfredResponse{Items: []alfredItem{invalidItem("参数错误", err.Error())}})
		return nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		writeAlfred(alfredResponse{Items: []alfredItem{invalidItem(
			"输入要翻译的文本",
			"默认自动判断中英方向；可用 ALFRED_TRANSLATE_TARGET 固定目标语言",
		)}})
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	result, err := translateWithCache(ctx, cfg, query)
	if err != nil {
		writeAlfred(alfredResponse{Items: []alfredItem{invalidItem("翻译失败", err.Error())}})
		return nil
	}

	subtitle := fmt.Sprintf("%s: %s -> %s | Enter 复制译文 | Cmd 复制原文",
		result.Provider,
		displayLang(result.SourceLang),
		displayLang(result.TargetLang),
	)
	writeAlfred(alfredResponse{
		Items: []alfredItem{
			{
				UID:      cacheKey(cfg, query),
				Title:    oneLine(result.Text),
				Subtitle: subtitle,
				Arg:      result.Text,
				Valid:    boolPtr(true),
				Text: &alfredText{
					Copy:      result.Text,
					LargeType: result.Text,
				},
				Mods: map[string]alfredMod{
					"cmd": {
						Arg:      query,
						Subtitle: "复制原文",
					},
				},
			},
		},
	})

	return nil
}

func runTranslate(args []string) error {
	cfg, query, err := parseFlags("translate", args)
	if err != nil {
		return err
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return errors.New("missing text to translate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	result, err := translateWithCache(ctx, cfg, query)
	if err != nil {
		return err
	}

	fmt.Println(result.Text)
	return nil
}

func parseFlags(name string, args []string) (config, string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := config{}
	fs.StringVar(&cfg.Provider, "provider", getenv("ALFRED_TRANSLATE_PROVIDER", "auto"), "provider: auto, deepl, google")
	fs.StringVar(&cfg.SourceLang, "source", getenv("ALFRED_TRANSLATE_SOURCE", "auto"), "source language")
	fs.StringVar(&cfg.TargetLang, "target", getenv("ALFRED_TRANSLATE_TARGET", "auto"), "target language")

	timeoutMS := fs.Int("timeout-ms", getenvInt("ALFRED_TRANSLATE_TIMEOUT_MS", int(defaultTimeout/time.Millisecond)), "request timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return cfg, "", err
	}

	cfg.Provider = strings.ToLower(strings.TrimSpace(cfg.Provider))
	cfg.SourceLang = normalizeLang(cfg.SourceLang)
	cfg.TargetLang = normalizeLang(cfg.TargetLang)
	cfg.Timeout = time.Duration(*timeoutMS) * time.Millisecond
	cfg.CacheDir = cacheDir()

	query := strings.Join(fs.Args(), " ")
	if cfg.TargetLang == "" || cfg.TargetLang == "auto" {
		cfg.TargetLang = autoTarget(query)
	}
	if cfg.SourceLang == "" {
		cfg.SourceLang = "auto"
	}
	if cfg.Provider == "" {
		cfg.Provider = "auto"
	}
	return cfg, query, nil
}

func translateWithCache(ctx context.Context, cfg config, text string) (translation, error) {
	if cached, ok := readCache(cfg, text); ok {
		return cached, nil
	}

	result, err := translate(ctx, cfg, text)
	if err != nil {
		return translation{}, err
	}
	writeCache(cfg, text, result)
	return result, nil
}

func translate(ctx context.Context, cfg config, text string) (translation, error) {
	switch cfg.Provider {
	case "auto":
		if os.Getenv("DEEPL_AUTH_KEY") != "" {
			if result, err := deeplTranslate(ctx, cfg, text); err == nil {
				return result, nil
			}
		}
		return googleTranslate(ctx, cfg, text)
	case "deepl":
		return deeplTranslate(ctx, cfg, text)
	case "google":
		return googleTranslate(ctx, cfg, text)
	default:
		return translation{}, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

func deeplTranslate(ctx context.Context, cfg config, text string) (translation, error) {
	key := os.Getenv("DEEPL_AUTH_KEY")
	if key == "" {
		return translation{}, errors.New("DEEPL_AUTH_KEY is required when provider is deepl")
	}

	apiURL := getenv("DEEPL_API_URL", "https://api-free.deepl.com/v2/translate")
	form := url.Values{}
	form.Set("auth_key", key)
	form.Set("text", text)
	form.Set("target_lang", deeplLang(cfg.TargetLang))
	if cfg.SourceLang != "" && cfg.SourceLang != "auto" {
		form.Set("source_lang", deeplLang(cfg.SourceLang))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return translation{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "alfred-translate/0.1")

	body, err := doRequest(req)
	if err != nil {
		return translation{}, err
	}

	var payload struct {
		Translations []struct {
			DetectedSourceLanguage string `json:"detected_source_language"`
			Text                   string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return translation{}, fmt.Errorf("decode deepl response: %w", err)
	}
	if len(payload.Translations) == 0 {
		return translation{}, errors.New("deepl returned no translations")
	}

	return translation{
		Text:       payload.Translations[0].Text,
		Provider:   "DeepL",
		SourceLang: strings.ToLower(payload.Translations[0].DetectedSourceLanguage),
		TargetLang: cfg.TargetLang,
	}, nil
}

func googleTranslate(ctx context.Context, cfg config, text string) (translation, error) {
	endpoint := getenv("GOOGLE_TRANSLATE_ENDPOINT", "https://translate.googleapis.com/translate_a/single")
	params := url.Values{}
	params.Set("client", "gtx")
	params.Set("sl", cfg.SourceLang)
	params.Set("tl", cfg.TargetLang)
	params.Set("dt", "t")
	params.Set("q", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return translation{}, err
	}
	req.Header.Set("User-Agent", "alfred-translate/0.1")

	body, err := doRequest(req)
	if err != nil {
		return translation{}, err
	}

	translated, detected, err := parseGoogleResponse(body)
	if err != nil {
		return translation{}, err
	}

	return translation{
		Text:       translated,
		Provider:   "Google",
		SourceLang: detected,
		TargetLang: cfg.TargetLang,
	}, nil
}

func doRequest(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", req.URL.Host, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func parseGoogleResponse(body []byte) (string, string, error) {
	var payload []any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", fmt.Errorf("decode google response: %w", err)
	}
	if len(payload) == 0 {
		return "", "", errors.New("google returned an empty response")
	}

	segments, ok := payload[0].([]any)
	if !ok {
		return "", "", errors.New("google response missing translation segments")
	}

	var builder strings.Builder
	for _, rawSegment := range segments {
		segment, ok := rawSegment.([]any)
		if !ok || len(segment) == 0 {
			continue
		}
		if text, ok := segment[0].(string); ok {
			builder.WriteString(text)
		}
	}

	translated := strings.TrimSpace(builder.String())
	if translated == "" {
		return "", "", errors.New("google returned an empty translation")
	}

	detected := "auto"
	if len(payload) > 2 {
		if lang, ok := payload[2].(string); ok && lang != "" {
			detected = normalizeLang(lang)
		}
	}

	return translated, detected, nil
}

func readCache(cfg config, text string) (translation, bool) {
	if cfg.CacheDir == "" {
		return translation{}, false
	}

	path := filepath.Join(cfg.CacheDir, cacheKey(cfg, text)+".json")
	stat, err := os.Stat(path)
	if err != nil || time.Since(stat.ModTime()) > cacheTTL {
		return translation{}, false
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return translation{}, false
	}

	var result translation
	if err := json.Unmarshal(body, &result); err != nil {
		return translation{}, false
	}
	return result, result.Text != ""
}

func writeCache(cfg config, text string, result translation) {
	if cfg.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return
	}

	body, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(cfg.CacheDir, cacheKey(cfg, text)+".json"), body, 0o644)
}

func cacheKey(cfg config, text string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		cfg.Provider,
		cfg.SourceLang,
		cfg.TargetLang,
		text,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func cacheDir() string {
	if dir := os.Getenv("alfred_workflow_cache"); dir != "" {
		return dir
	}
	if dir := os.Getenv("ALFRED_TRANSLATE_CACHE_DIR"); dir != "" {
		return dir
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "alfred-translate")
}

func autoTarget(text string) string {
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			return "en"
		}
	}
	return "zh-CN"
}

func normalizeLang(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return ""
	}
	switch strings.ToLower(lang) {
	case "zh", "zh_cn", "zh-cn", "cn", "chinese":
		return "zh-CN"
	case "zh_tw", "zh-tw":
		return "zh-TW"
	case "en", "english":
		return "en"
	case "ja", "jp", "japanese":
		return "ja"
	case "ko", "kr", "korean":
		return "ko"
	default:
		return lang
	}
}

func deeplLang(lang string) string {
	switch normalizeLang(lang) {
	case "zh-CN", "zh-TW":
		return "ZH"
	case "en":
		return "EN-US"
	default:
		return strings.ToUpper(lang)
	}
}

func displayLang(lang string) string {
	if lang == "" {
		return "auto"
	}
	return lang
}

func oneLine(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text
	}
	return strings.Join(fields, " ")
}

func invalidItem(title, subtitle string) alfredItem {
	return alfredItem{
		Title:    title,
		Subtitle: subtitle,
		Valid:    boolPtr(false),
	}
}

func writeAlfred(response alfredResponse) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(response)
}

func writeUsage(out io.Writer) {
	fmt.Fprintln(out, "usage:")
	fmt.Fprintln(out, "  alfred-translate filter [flags] <text>")
	fmt.Fprintln(out, "  alfred-translate translate [flags] <text>")
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func boolPtr(value bool) *bool {
	return &value
}
