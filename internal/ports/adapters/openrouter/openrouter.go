package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/domain/highlights"
	"github.com/forPelevin/hlcut/internal/types"
)

type Adapter struct {
	key     string
	model   string
	baseURL string
	client  *http.Client
}

type transcriptTiming struct {
	words   []timedWord
	segEnds []time.Duration
}

type timedWord struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

const (
	requestTimeout = 90 * time.Second
)

func New(apiKey, model, baseURL string) *Adapter {
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}
	baseURL = normalizeBaseURL(baseURL)
	return &Adapter{key: apiKey, model: model, baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (a *Adapter) Refine(
	ctx context.Context,
	tr types.Transcript,
	cands []types.Candidate,
	clipsN int,
) ([]types.ClipSpec, error) {
	_ = tr // reserved for future

	if clipsN <= 0 || len(cands) == 0 {
		return nil, nil
	}
	minClip, maxClip := highlights.DurationBounds()
	if maxClip <= 0 || maxClip < minClip {
		return nil, nil
	}
	timing := collectTranscriptTiming(tr)

	top := selectPromptCandidates(cands, 80)
	if len(top) == 0 {
		return nil, nil
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
		"maxClips":   clipsN,
		"minSec":     minClip.Seconds(),
		"maxSec":     maxClip.Seconds(),
		"candidates": arr,
	}
	pb, err := json.Marshal(prompt)
	if err != nil {
		return nil, fmt.Errorf("marshal prompt: %w", err)
	}

	// strict schema: select clips with start/end and metadata.
	payload := map[string]any{
		"model":  a.model,
		"stream": false,
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

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	url := a.baseURL + "/api/v1/chat/completions"

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("openrouter timeout after %s (model=%s)", requestTimeout, a.model)
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rb, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("openrouter status %d and read body failed: %v", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("openrouter status %d: %s", resp.StatusCode, truncate(redactSecrets(string(rb), a.key), 400))
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
		return fallbackHighlights(top, clipsN, minClip, maxClip, timing), nil
	}

	content, err := messageContentToString(raw.Choices[0].Message.Content)
	if err != nil {
		return fallbackHighlights(top, clipsN, minClip, maxClip, timing), nil
	}

	clean, err := extractJSONObject(content)
	if err != nil {
		return fallbackHighlights(top, clipsN, minClip, maxClip, timing), nil
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
		return fallbackHighlights(top, clipsN, minClip, maxClip, timing), nil
	}

	res := make([]types.ClipSpec, 0, min(len(out.Clips), clipsN))
	for _, c := range out.Clips {
		st, en, ok := normalizeClipRange(c.Idx, c.StartSec, c.EndSec, top, minClip, maxClip, timing)
		if !ok {
			continue
		}
		if !isDistinct(res, st, en, 2*time.Second) {
			continue
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

	// If model failed to return valid clips, keep the pipeline useful with deterministic fallback.
	if len(res) == 0 {
		res = fallbackHighlights(top, clipsN, minClip, maxClip, timing)
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
			"Prefer clips that are both informative and hooky. " +
			"Clips must be distinct scenes with no overlaps/intersections and can be anywhere from 0 to maxClips total. " +
			"Each clip duration must be between minSec and maxSec. " +
			"Clips must start cleanly and end on a complete thought, ideally right after a payoff/peak or hook explanation." +
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

var (
	bearerTokenRE = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._-]+\b`)
	authHeaderRE  = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)([^\n\r,;]+)`)
	apiKeyFieldRE = regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)([^\n\r,;]+)`)
)

func redactSecrets(s, apiKey string) string {
	if s == "" {
		return s
	}
	out := s
	if apiKey != "" {
		out = strings.ReplaceAll(out, apiKey, "[REDACTED]")
	}
	out = bearerTokenRE.ReplaceAllString(out, "Bearer [REDACTED]")
	out = authHeaderRE.ReplaceAllString(out, "${1}[REDACTED]")
	out = apiKeyFieldRE.ReplaceAllString(out, "${1}[REDACTED]")
	return out
}

func fallbackHighlights(
	cands []types.Candidate,
	clipsN int,
	minClip, maxClip time.Duration,
	timing transcriptTiming,
) []types.ClipSpec {
	if clipsN <= 0 {
		return nil
	}

	best := make([]types.Candidate, len(cands))
	copy(best, cands)
	sort.Slice(best, func(i, j int) bool {
		s1 := best[i].InfoScore + best[i].HookScore
		s2 := best[j].InfoScore + best[j].HookScore
		if s1 == s2 {
			return best[i].Start < best[j].Start
		}
		return s1 > s2
	})

	out := make([]types.ClipSpec, 0, clipsN)
	for _, c := range best {
		if len(out) >= clipsN {
			break
		}
		st, en, ok := normalizeClipDur(c.Start, c.End, minClip, maxClip, timing)
		if !ok {
			continue
		}
		if !isDistinct(out, st, en, 2*time.Second) {
			continue
		}
		caption := strings.TrimSpace(c.Text)
		if caption == "" {
			caption = "Highlight"
		}
		out = append(out, types.ClipSpec{
			Start:   st,
			End:     en,
			Title:   "Highlight",
			Caption: caption,
			Reason:  "fallback",
		})
	}
	return out
}

func selectPromptCandidates(cands []types.Candidate, limit int) []types.Candidate {
	if len(cands) == 0 || limit <= 0 {
		return nil
	}

	best := make([]types.Candidate, len(cands))
	copy(best, cands)
	sort.Slice(best, func(i, j int) bool {
		s1 := best[i].InfoScore + best[i].HookScore
		s2 := best[j].InfoScore + best[j].HookScore
		if s1 == s2 {
			return best[i].Start < best[j].Start
		}
		return s1 > s2
	})

	out := make([]types.Candidate, 0, limit)
	for _, c := range best {
		if len(out) >= limit {
			break
		}
		if !isDistinctCandidate(out, c.Start, c.End, 2*time.Second) {
			continue
		}
		out = append(out, c)
	}

	if len(out) < limit {
		for _, c := range cands {
			if len(out) >= limit {
				break
			}
			if !isDistinctCandidate(out, c.Start, c.End, 2*time.Second) {
				continue
			}
			out = append(out, c)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

func normalizeClipRange(
	idx int,
	startSec float64,
	endSec float64,
	cands []types.Candidate,
	minClip time.Duration,
	maxClip time.Duration,
	timing transcriptTiming,
) (time.Duration, time.Duration, bool) {
	st := time.Duration(startSec * float64(time.Second))
	en := time.Duration(endSec * float64(time.Second))
	if st < 0 {
		st = 0
	}

	if st, en, ok := normalizeClipDur(st, en, minClip, maxClip, timing); ok {
		return st, en, true
	}

	if idx < 0 || idx >= len(cands) {
		return 0, 0, false
	}
	return normalizeClipDur(cands[idx].Start, cands[idx].End, minClip, maxClip, timing)
}

func normalizeClipDur(
	st, en, minClip, maxClip time.Duration,
	timing transcriptTiming,
) (time.Duration, time.Duration, bool) {
	if en <= st {
		return 0, 0, false
	}
	maxEnd := st + maxClip
	if en > maxEnd {
		en = maxEnd
	}
	minEnd := st + minClip
	if en < minEnd {
		return 0, 0, false
	}

	// Prefer a natural stop close to the requested end (sentence ending or a pause),
	// while respecting duration limits.
	smoothEnd := chooseNaturalEnd(timing, st, en, minEnd, maxEnd)
	if smoothEnd < minEnd {
		return 0, 0, false
	}
	if smoothEnd > maxEnd {
		smoothEnd = maxEnd
	}
	en = smoothEnd

	return st, en, true
}

func collectTranscriptTiming(tr types.Transcript) transcriptTiming {
	t := transcriptTiming{
		words:   make([]timedWord, 0, 1024),
		segEnds: make([]time.Duration, 0, len(tr.Segments)),
	}
	for _, s := range tr.Segments {
		se := dur(s.End)
		if se > 0 {
			t.segEnds = append(t.segEnds, se)
		}
		for _, w := range s.Words {
			ws := dur(w.Start)
			we := dur(w.End)
			if we <= ws {
				continue
			}
			txt := strings.TrimSpace(w.Word)
			if txt == "" {
				continue
			}
			t.words = append(t.words, timedWord{
				Start: ws,
				End:   we,
				Text:  txt,
			})
		}
	}
	sort.Slice(t.words, func(i, j int) bool {
		if t.words[i].Start == t.words[j].Start {
			return t.words[i].End < t.words[j].End
		}
		return t.words[i].Start < t.words[j].Start
	})
	sort.Slice(t.segEnds, func(i, j int) bool {
		return t.segEnds[i] < t.segEnds[j]
	})
	return t
}

func chooseNaturalEnd(
	t transcriptTiming,
	start, requestedEnd, minEnd, maxEnd time.Duration,
) time.Duration {
	if requestedEnd < minEnd {
		requestedEnd = minEnd
	}
	if requestedEnd > maxEnd {
		requestedEnd = maxEnd
	}

	// Allow tiny extension to finish the current sentence if headroom exists.
	searchEnd := requestedEnd
	extend := 2 * time.Second
	if searchEnd+extend < maxEnd {
		searchEnd += extend
	} else {
		searchEnd = maxEnd
	}

	// 1) Score sentence boundaries and choose the most complete logical ending.
	if end, ok := bestSentenceEnd(t.words, start, requestedEnd, minEnd, searchEnd); ok {
		return end
	}

	// 2) Fallback to a pause boundary.
	const pauseThreshold = 350 * time.Millisecond
	pauseLookback := 8 * time.Second
	pauseStart := searchEnd - pauseLookback
	if pauseStart < minEnd {
		pauseStart = minEnd
	}
	var (
		bestPause    time.Duration
		bestPauseEnd time.Duration
	)
	for i := 0; i+1 < len(t.words); i++ {
		cur := t.words[i]
		next := t.words[i+1]
		if cur.End < pauseStart || cur.End > searchEnd {
			continue
		}
		if next.Start <= cur.End {
			continue
		}
		pause := next.Start - cur.End
		if pause >= pauseThreshold && pause > bestPause {
			bestPause = pause
			bestPauseEnd = cur.End
		}
	}
	if bestPauseEnd >= minEnd {
		return bestPauseEnd
	}

	// 3) Latest segment end before tail.
	var segEnd time.Duration
	for _, se := range t.segEnds {
		if se < minEnd || se > searchEnd {
			continue
		}
		if se > segEnd {
			segEnd = se
		}
	}
	if segEnd >= minEnd {
		return segEnd
	}

	// 4) Latest known word end.
	var wordEnd time.Duration
	for _, w := range t.words {
		if w.End < minEnd || w.End > searchEnd {
			continue
		}
		if w.End > wordEnd {
			wordEnd = w.End
		}
	}
	if wordEnd >= minEnd {
		return wordEnd
	}

	return requestedEnd
}

type sentenceEndCandidate struct {
	End         time.Duration
	Words       int
	LastWord    string
	Sentence    string
	NextWord    string
	PauseAfter  time.Duration
	HasTerminal bool
}

func bestSentenceEnd(
	words []timedWord,
	clipStart, requestedEnd, minEnd, searchEnd time.Duration,
) (time.Duration, bool) {
	cands := collectSentenceEndCandidates(words, clipStart, minEnd, searchEnd)
	if len(cands) == 0 {
		return 0, false
	}

	bestIdx := -1
	bestScore := -1e9
	for i := range cands {
		score := scoreSentenceEnd(cands[i], requestedEnd)
		if score > bestScore || (score == bestScore && cands[i].End > cands[bestIdx].End) {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return 0, false
	}
	return cands[bestIdx].End, true
}

func collectSentenceEndCandidates(
	words []timedWord,
	clipStart, minEnd, searchEnd time.Duration,
) []sentenceEndCandidate {
	out := make([]sentenceEndCandidate, 0, 16)
	for i := range words {
		w := words[i]
		if w.End < minEnd || w.End > searchEnd || !hasTerminalPunctuation(w.Text) {
			continue
		}

		sentenceStartIdx := 0
		for j := i - 1; j >= 0; j-- {
			if words[j].End <= clipStart {
				sentenceStartIdx = j + 1
				break
			}
			if hasTerminalPunctuation(words[j].Text) {
				sentenceStartIdx = j + 1
				break
			}
		}

		parts := make([]string, 0, i-sentenceStartIdx+1)
		lastWord := ""
		wordCount := 0
		for k := sentenceStartIdx; k <= i; k++ {
			if words[k].End <= clipStart {
				continue
			}
			txt := strings.TrimSpace(words[k].Text)
			if txt == "" {
				continue
			}
			parts = append(parts, txt)
			norm := normalizeToken(txt)
			if norm != "" {
				wordCount++
				lastWord = norm
			}
		}
		if len(parts) == 0 {
			continue
		}

		nextWord := ""
		pauseAfter := time.Duration(0)
		if i+1 < len(words) {
			if words[i+1].Start > w.End {
				pauseAfter = words[i+1].Start - w.End
			}
			nextWord = normalizeToken(words[i+1].Text)
		}

		out = append(out, sentenceEndCandidate{
			End:         w.End,
			Words:       wordCount,
			LastWord:    lastWord,
			Sentence:    strings.ToLower(strings.Join(parts, " ")),
			NextWord:    nextWord,
			PauseAfter:  pauseAfter,
			HasTerminal: true,
		})
	}
	return out
}

func scoreSentenceEnd(c sentenceEndCandidate, requestedEnd time.Duration) float64 {
	// Keep close to the model-requested end unless a later/earlier boundary is clearly better.
	distScore := -0.30 * absDuration(c.End-requestedEnd).Seconds()
	score := distScore
	hasClosure := hasClosureCue(c.Sentence)

	switch {
	case c.Words >= 8:
		score += 1.1
	case c.Words >= 5:
		score += 0.5
	case c.Words < 4:
		score -= 0.8
	}

	switch {
	case c.PauseAfter >= 450*time.Millisecond:
		score += 1.0
	case c.PauseAfter >= 250*time.Millisecond:
		score += 0.4
	case c.PauseAfter < 120*time.Millisecond:
		score -= 0.35
	}

	if hasClosure {
		score += 1.1
	}
	if isDanglingTail(c.LastWord) {
		score -= 2.0
	}
	if strings.HasSuffix(c.Sentence, "?") && c.PauseAfter < 450*time.Millisecond {
		score -= 2.4
	}
	if isContinuationStart(c.NextWord) && c.PauseAfter < 350*time.Millisecond {
		score -= 0.8
	}
	if c.PauseAfter < 120*time.Millisecond && c.NextWord != "" {
		score -= 0.8
	}
	if c.Words < 5 && !hasClosure && c.PauseAfter < 200*time.Millisecond {
		score -= 0.9
	}

	return score
}

func hasClosureCue(s string) bool {
	cues := []string{
		"that's it",
		"that is it",
		"that's why",
		"that's how",
		"there you go",
		"we're out",
		"we are out",
		"i'm out",
		"i am out",
		"goodbye",
		"finally",
		"done",
		"finished",
		"let's go",
		"lets go",
		"we won",
		"i won",
		"you won",
		"we did it",
	}
	for _, cue := range cues {
		if strings.Contains(s, cue) {
			return true
		}
	}
	return false
}

func isDanglingTail(lastWord string) bool {
	if lastWord == "" {
		return true
	}
	switch lastWord {
	case "and", "but", "or", "so", "because", "if", "when", "then",
		"to", "of", "for", "with", "from", "into", "onto",
		"the", "a", "an", "this", "that", "these", "those",
		"my", "your", "our", "their", "his", "her", "its":
		return true
	default:
		return false
	}
}

func isContinuationStart(word string) bool {
	switch word {
	case "and", "but", "or", "so", "because", "then", "if", "when", "while", "that":
		return true
	default:
		return false
	}
}

func normalizeToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	trimRunes := `"'` + "`" + "[](){}.,!?;:"
	s = strings.Trim(s, trimRunes)
	return s
}

func hasTerminalPunctuation(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	trimTail := `"'` + "`" + ")]}"
	for len(s) > 0 && strings.ContainsRune(trimTail, rune(s[len(s)-1])) {
		s = s[:len(s)-1]
	}
	if s == "" {
		return false
	}
	last := s[len(s)-1]
	return last == '.' || last == '!' || last == '?'
}

func dur(sec float64) time.Duration {
	return time.Duration(sec * float64(time.Second))
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func isDistinct(existing []types.ClipSpec, st, en, minGap time.Duration) bool {
	for _, e := range existing {
		if st < e.End+minGap && en > e.Start-minGap {
			return false
		}
	}
	return true
}

func isDistinctCandidate(existing []types.Candidate, st, en, minGap time.Duration) bool {
	for _, e := range existing {
		if st < e.End+minGap && en > e.Start-minGap {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
