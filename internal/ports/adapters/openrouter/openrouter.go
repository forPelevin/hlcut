package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	return &Adapter{key: apiKey, model: model, baseURL: baseURL, client: &http.Client{Timeout: 90 * time.Second}}
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

	content, ok := raw.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("openrouter: unexpected content type")
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
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, err
	}

	res := make([]types.ClipSpec, 0, len(out.Clips))
	for _, c := range out.Clips {
		st := time.Duration(c.StartSec * float64(time.Second))
		en := time.Duration(c.EndSec * float64(time.Second))
		if en <= st {
			continue
		}
		if en-st > maxClip {
			en = st + maxClip
		}
		res = append(res, types.ClipSpec{Start: st, End: en, Title: c.Title, Caption: c.Caption, Tags: c.Tags, Reason: c.Reason})
		if len(res) >= clipsN {
			break
		}
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("openrouter: no valid clips")
	}
	return res, nil
}

func buildPrompt(candsJSON []byte) []byte {
	return []byte(
		"Select the best highlight clips from the candidate list. Return strictly valid JSON matching the provided schema. " +
			"Prefer clips that are both informative and hooky. Avoid near-duplicates. " +
			"Clips must start cleanly and end on a complete thought." +
			"\n\nCandidates JSON:\n" + string(candsJSON),
	)
}
