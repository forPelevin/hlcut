package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

type Adapter struct {
	key     string
	model   string
	baseURL string
	client  *http.Client
}

func New(apiKey, model, baseURL string) *Adapter {
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}
	if baseURL == "" {
		baseURL = "https://openrouter.ai"
	}
	return &Adapter{key: apiKey, model: model, baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (a *Adapter) Refine(ctx context.Context, tr types.Transcript, cands []types.Candidate, clipsN int, maxClip time.Duration) ([]types.ClipSpec, error) {
	_ = tr // reserved for future (transcript summary/context)

	// Keep prompt bounded.
	top := cands
	if len(top) > 80 {
		top = top[:80]
	}

	type cand struct {
		Idx      int     `json:"idx"`
		StartSec float64 `json:"start_sec"`
		EndSec   float64 `json:"end_sec"`
		Text     string  `json:"text"`
		Info     float64 `json:"info"`
		Hook     float64 `json:"hook"`
	}
	arr := make([]cand, 0, len(top))
	for i, c := range top {
		arr = append(arr, cand{Idx: i, StartSec: c.Start.Seconds(), EndSec: c.End.Seconds(), Text: c.Text, Info: c.InfoScore, Hook: c.HookScore})
	}

	prompt := map[string]any{
		"clipsN":     clipsN,
		"maxSec":     maxClip.Seconds(),
		"candidates": arr,
	}
	pb, _ := json.Marshal(prompt)

	// strict schema: select clips with start/end and metadata.
	payload := map[string]any{
		"model": a.model,
		"messages": []map[string]any{
			{"role": "user", "content": string(buildPrompt(pb))},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": "hlcut_refine",
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"clips": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"idx":       map[string]any{"type": "integer"},
									"start_sec": map[string]any{"type": "number"},
									"end_sec":   map[string]any{"type": "number"},
									"title":     map[string]any{"type": "string"},
									"caption":   map[string]any{"type": "string"},
									"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
									"reason":    map[string]any{"type": "string"},
								},
								"required": []string{"idx", "start_sec", "end_sec", "title", "caption", "tags", "reason"},
							},
						},
					},
					"required": []string{"clips"},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	url := a.baseURL + "/api/v1/chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter status %d: %s", resp.StatusCode, string(rb))
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("openrouter: empty choices")
	}

	content, err := messageContentToString(raw.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}

	clean, err := extractJSONObject(content)
	if err != nil {
		return nil, err
	}

	var out struct {
		Clips []struct {
			Idx      int      `json:"idx"`
			StartSec float64  `json:"start_sec"`
			EndSec   float64  `json:"end_sec"`
			Title    string   `json:"title"`
			Caption  string   `json:"caption"`
			Tags     []string `json:"tags"`
			Reason   string   `json:"reason"`
		} `json:"clips"`
	}
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, err
	}

	res := make([]types.ClipSpec, 0, len(out.Clips))
	for _, c := range out.Clips {
		st := time.Duration(c.StartSec * float64(time.Second))
		en := time.Duration(c.EndSec * float64(time.Second))

		// If the model returned invalid timing, fall back to candidate boundaries by idx.
		if (en <= st) && c.Idx >= 0 && c.Idx < len(top) {
			st = top[c.Idx].Start
			en = top[c.Idx].End
		}
		if en <= st {
			continue
		}
		if en-st > maxClip {
			en = st + maxClip
		}

		title := strings.TrimSpace(c.Title)
		caption := strings.TrimSpace(c.Caption)
		if title == "" {
			title = "Highlight"
		}
		if caption == "" {
			caption = title
		}

		res = append(res, types.ClipSpec{Start: st, End: en, Title: title, Caption: caption, Tags: c.Tags, Reason: c.Reason})
		if len(res) >= clipsN {
			break
		}
	}
	// If model returned fewer clips than requested, pad deterministically using best-scoring candidates.
	if len(res) < clipsN {
		best := make([]types.Candidate, len(top))
		copy(best, top)
		sort.Slice(best, func(i, j int) bool {
			s1 := best[i].InfoScore + best[i].HookScore
			s2 := best[j].InfoScore + best[j].HookScore
			if s1 == s2 {
				return best[i].Start < best[j].Start
			}
			return s1 > s2
		})

		for _, c := range best {
			if len(res) >= clipsN {
				break
			}
			st, en := c.Start, c.End
			if en-st > maxClip {
				en = st + maxClip
			}
			if !isNonOverlapping(res, st, en) {
				continue
			}
			res = append(res, types.ClipSpec{Start: st, End: en, Title: "Highlight", Caption: strings.TrimSpace(c.Text), Tags: nil, Reason: "pad"})
		}
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("openrouter: no valid clips")
	}
	if len(res) > clipsN {
		res = res[:clipsN]
	}
	return res, nil
}

func buildPrompt(candsJSON []byte) []byte {
	return []byte(
		"Select the best highlight clips from the candidate list. " +
			"Return strictly valid JSON (no markdown, no code fences) matching the provided schema. " +
			"Prefer clips that are both informative and hooky. Avoid near-duplicates. " +
			"Clips must start cleanly and end on a complete thought." +
			"\n\nCandidates JSON:\n" + string(candsJSON),
	)
}

func messageContentToString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case []any:
		// Some providers return an array of {type,text} parts.
		var b strings.Builder
		for _, it := range x {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := m["text"].(string); ok {
				b.WriteString(t)
			}
		}
		s := b.String()
		if strings.TrimSpace(s) == "" {
			return "", errors.New("openrouter: empty content")
		}
		return s, nil
	default:
		return "", fmt.Errorf("openrouter: unexpected content type %T", v)
	}
}

func extractJSONObject(s string) (string, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return "", errors.New("openrouter: empty content")
	}

	// Strip markdown code fences.
	if strings.HasPrefix(t, "```") {
		// Remove opening fence line.
		if i := strings.Index(t, "\n"); i >= 0 {
			t = t[i+1:]
		}
		// Remove trailing fence.
		if j := strings.LastIndex(t, "```"); j >= 0 {
			t = t[:j]
		}
		t = strings.TrimSpace(t)
	}

	// Best-effort: take the first JSON object found.
	start := strings.Index(t, "{")
	end := strings.LastIndex(t, "}")
	if start >= 0 && end > start {
		return t[start : end+1], nil
	}

	return "", fmt.Errorf("openrouter: could not locate JSON object in: %q", truncate(t, 200))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func isNonOverlapping(existing []types.ClipSpec, st, en time.Duration) bool {
	for _, e := range existing {
		// consider overlaps or near-duplicates (start within 1s)
		if absDur(e.Start-st) < time.Second {
			return false
		}
		if st < e.End && en > e.Start {
			return false
		}
	}
	return true
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
