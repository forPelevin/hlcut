package types

import "time"

type Transcript struct {
	Segments []Segment `json:"segments"`
}

type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Words []Word  `json:"words,omitempty"`
}

type Word struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Word  string  `json:"word"`
}

type Candidate struct {
	Start time.Duration
	End   time.Duration
	Text  string

	InfoScore float64
	HookScore float64
}

type ClipSpec struct {
	Start   time.Duration
	End     time.Duration
	Title   string
	Caption string
	Tags    []string
	Reason  string
}

type Manifest struct {
	Input string         `json:"input"`
	Clips []ManifestClip `json:"clips"`
}

type ManifestClip struct {
	ID        string   `json:"id"`
	StartSec  float64  `json:"start_sec"`
	EndSec    float64  `json:"end_sec"`
	InfoScore float64  `json:"info_score"`
	HookScore float64  `json:"hook_score"`
	Text      string   `json:"text"`
	File      string   `json:"file"`
	Subtitles string   `json:"subtitles"`
	Title     string   `json:"title"`
	Caption   string   `json:"caption"`
	Tags      []string `json:"tags"`
}
