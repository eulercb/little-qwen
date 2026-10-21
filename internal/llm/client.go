// Package llm is a minimal streaming client for OpenAI-compatible chat
// completion endpoints, tuned for a slow, remote, single-lane local model
// served behind a tunnel (see docs/ENDPOINT.md).
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is one chat message. ReasoningContent carries the model's thinking
// and is sent back verbatim so earlier context never changes (prefix stability).
type Message struct {
	Role             string     `json:"role"`
	Content          *string    `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`
}

// Text returns the message content or "".
func (m Message) Text() string {
	if m.Content == nil {
		return ""
	}
	return *m.Content
}

// Str is a helper for building Message.Content.
func Str(s string) *string { return &s }

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is an OpenAI function tool definition.
type Tool struct {
	Type     string      `json:"type"`
	Function ToolFuncDef `json:"function"`
}

type ToolFuncDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Request holds the knobs the harness varies per call.
type Request struct {
	Messages        []Message
	Tools           []Tool
	MaxTokens       int
	ReasoningEffort string // xhigh | medium | low | "" (server default)
	DisableThinking bool
	Temperature     *float64
	TopP            *float64
	PresencePenalty *float64
}

// Usage mirrors the server's usage + metrics blocks.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`
	PrefillTokens    int `json:"prefill_tokens"` // tokens actually prefilled (prompt - cache hit)
}

// Response is the assembled result of one streamed completion.
type Response struct {
	Message      Message
	FinishReason string
	Usage        Usage
	LatencyMS    int64
	TTFTMS       int64
	CacheStatus  string
	Retries      int
}

// Client talks to one endpoint.
type Client struct {
	BaseURL string
	Model   string
	HTTP    *http.Client
	// IdleTimeout aborts a stream when no bytes arrive for this long.
	IdleTimeout time.Duration
	// MaxWait bounds the total time spent retrying an unavailable endpoint.
	MaxWait time.Duration
	Log     func(format string, args ...any)
}

// New builds a client from the environment: REMOTE_LLM_ENDPOINT (or
// QH_ENDPOINT) and QH_MODEL.
func New() (*Client, error) {
	base := os.Getenv("QH_ENDPOINT")
	if base == "" {
		base = os.Getenv("REMOTE_LLM_ENDPOINT")
	}
	if base == "" {
		return nil, errors.New("no endpoint: set REMOTE_LLM_ENDPOINT or QH_ENDPOINT")
	}
	base = strings.TrimRight(base, "/")
	base = strings.TrimSuffix(base, "/v1")
	model := os.Getenv("QH_MODEL")
	if model == "" {
		model = "mlx-community/Qwen3.8-27B-4bit"
	}
	return &Client{
		BaseURL:     base,
		Model:       model,
		HTTP:        &http.Client{Timeout: 0},
		IdleTimeout: 240 * time.Second,
		MaxWait:     45 * time.Minute,
		Log:         func(string, ...any) {},
	}, nil
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type wireRequest struct {
	Model              string            `json:"model"`
	Messages           []Message         `json:"messages"`
	Tools              []Tool            `json:"tools,omitempty"`
	MaxTokens          int               `json:"max_tokens,omitempty"`
	Stream             bool              `json:"stream"`
	StreamOptions      wireStreamOptions `json:"stream_options"`
	ReasoningEffort    string            `json:"reasoning_effort,omitempty"`
	ChatTemplateKwargs map[string]any    `json:"chat_template_kwargs,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	TopK               int               `json:"top_k,omitempty"`
	PresencePenalty    *float64          `json:"presence_penalty,omitempty"`
}

type wireChunk struct {
	Choices []struct {
		Delta struct {
			Content          *string `json:"content"`
			ReasoningContent *string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Metrics *struct {
		Prefill struct {
			Tokens int `json:"tokens"`
		} `json:"prefill"`
		Cache struct {
			Status        string `json:"status"`
			MatchedTokens int    `json:"matched_tokens"`
		} `json:"cache"`
	} `json:"metrics"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ErrContextOverflow is returned when the server rejects the prompt as too long.
var ErrContextOverflow = errors.New("context overflow")

type retryable struct{ err error }

func (r retryable) Error() string { return r.err.Error() }

// Chat performs one streamed completion, retrying transient failures
// (tunnel 5xx, connection drops, idle stalls) with backoff up to MaxWait.
func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()
	backoff := 5 * time.Second
	retries := 0
	for {
		resp, err := c.once(ctx, req)
		if err == nil {
			resp.Retries = retries
			return resp, nil
		}
		var rt retryable
		if !errors.As(err, &rt) {
			return nil, err
		}
		if time.Since(start) > c.MaxWait {
			return nil, fmt.Errorf("endpoint unavailable for %s: %w", c.MaxWait, err)
		}
		retries++
		c.Log("llm: retry %d after error: %v (sleep %s)", retries, err, backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) once(ctx context.Context, req Request) (*Response, error) {
	wr := wireRequest{
		Model:           c.Model,
		Messages:        req.Messages,
		Tools:           req.Tools,
		MaxTokens:       req.MaxTokens,
		Stream:          true,
		StreamOptions:   wireStreamOptions{IncludeUsage: true},
		ReasoningEffort: req.ReasoningEffort,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		PresencePenalty: req.PresencePenalty,
	}
	if req.DisableThinking {
		wr.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
		wr.ReasoningEffort = ""
	}
	body, err := json.Marshal(wr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	hreq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
	t0 := time.Now()
	hresp, err := c.HTTP.Do(hreq)
	if err != nil {
		if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
			return nil, err
		}
		return nil, retryable{err}
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(hresp.Body, 4096))
		msg := string(b)
		if hresp.StatusCode == 400 && (strings.Contains(msg, "context") || strings.Contains(msg, "too long") || strings.Contains(msg, "maximum")) {
			return nil, fmt.Errorf("%w: %s", ErrContextOverflow, trunc(msg, 300))
		}
		if hresp.StatusCode >= 500 || hresp.StatusCode == 429 || hresp.StatusCode == 408 || hresp.StatusCode == 403 && strings.Contains(msg, "cloudflare") {
			return nil, retryable{fmt.Errorf("HTTP %d: %s", hresp.StatusCode, trunc(stripHTML(msg), 200))}
		}
		return nil, fmt.Errorf("HTTP %d: %s", hresp.StatusCode, trunc(msg, 500))
	}

	// Idle watchdog: cancel the request if no line arrives for IdleTimeout.
	activity := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	var idled bool
	go func() {
		t := time.NewTimer(c.IdleTimeout)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-activity:
				if !t.Stop() {
					<-t.C
				}
				t.Reset(c.IdleTimeout)
			case <-t.C:
				idled = true
				cancel()
				return
			}
		}
	}()

	out := &Response{}
	var content, reasoning strings.Builder
	sawContent := false
	type tcAcc struct {
		id, name string
		args     strings.Builder
	}
	var tcs []*tcAcc
	sc := bufio.NewScanner(hresp.Body)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	gotDone := false
	for sc.Scan() {
		select {
		case activity <- struct{}{}:
		default:
		}
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			gotDone = true
			break
		}
		var ch wireChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			continue
		}
		if ch.Error != nil {
			msg := ch.Error.Message
			if strings.Contains(strings.ToLower(msg), "context") {
				return nil, fmt.Errorf("%w: %s", ErrContextOverflow, msg)
			}
			return nil, retryable{fmt.Errorf("stream error: %s", msg)}
		}
		for _, choice := range ch.Choices {
			d := choice.Delta
			if d.ReasoningContent != nil && *d.ReasoningContent != "" {
				if out.TTFTMS == 0 {
					out.TTFTMS = time.Since(t0).Milliseconds()
				}
				reasoning.WriteString(*d.ReasoningContent)
			}
			if d.Content != nil && *d.Content != "" {
				if out.TTFTMS == 0 {
					out.TTFTMS = time.Since(t0).Milliseconds()
				}
				sawContent = true
				content.WriteString(*d.Content)
			}
			for _, tc := range d.ToolCalls {
				if out.TTFTMS == 0 {
					out.TTFTMS = time.Since(t0).Milliseconds()
				}
				for len(tcs) <= tc.Index {
					tcs = append(tcs, &tcAcc{})
				}
				a := tcs[tc.Index]
				if tc.ID != "" {
					a.id = tc.ID
				}
				if tc.Function.Name != "" {
					a.name += tc.Function.Name
				}
				a.args.WriteString(tc.Function.Arguments)
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				out.FinishReason = *choice.FinishReason
			}
		}
		if ch.Usage != nil {
			out.Usage.PromptTokens = ch.Usage.PromptTokens
			out.Usage.CompletionTokens = ch.Usage.CompletionTokens
			out.Usage.CachedTokens = ch.Usage.PromptTokensDetails.CachedTokens
			out.Usage.ReasoningTokens = ch.Usage.CompletionTokensDetails.ReasoningTokens
		}
		if ch.Metrics != nil {
			out.Usage.PrefillTokens = ch.Metrics.Prefill.Tokens
			out.CacheStatus = ch.Metrics.Cache.Status
		}
	}
	if err := sc.Err(); err != nil || (!gotDone && out.FinishReason == "") {
		if idled {
			return nil, retryable{fmt.Errorf("stream idle for %s", c.IdleTimeout)}
		}
		if err == nil {
			err = errors.New("stream ended without finish_reason")
		}
		return nil, retryable{err}
	}
	out.LatencyMS = time.Since(t0).Milliseconds()
	msg := Message{Role: "assistant", ReasoningContent: reasoning.String()}
	if sawContent {
		msg.Content = Str(content.String())
	}
	for i, a := range tcs {
		id := a.id
		if id == "" {
			id = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)
		}
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: id, Type: "function", Function: FunctionCall{Name: a.name, Arguments: a.args.String()}})
	}
	out.Message = msg
	return out, nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stripHTML(s string) string {
	if i := strings.Index(s, "<title>"); i >= 0 {
		if j := strings.Index(s[i:], "</title>"); j > 0 {
			return s[i+7 : i+j]
		}
	}
	return s
}
