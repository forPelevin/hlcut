package subtitles

import (
	"fmt"
	"strings"
	"time"

	"github.com/forPelevin/hlcut/internal/types"
)

func RenderTikTokASS(tr types.Transcript, start, end time.Duration) (string, error) {
	words := collectWords(tr, start, end)
	if len(words) == 0 {
		// Fallback keeps subtitle rendering robust when ASR has segment text but
		// no usable per-word timestamps.
		text := collectSegmentText(tr, start, end)
		return renderASSPlain(text, end-start), nil
	}
	// Karaoke mode is preferred for readability and pacing in short-form clips.
	lines := packWords(words)
	return renderASSKaraoke(lines), nil
}

type wword struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

type line struct {
	Start time.Duration
	End   time.Duration
	Words []wword
}

func collectWords(tr types.Transcript, start, end time.Duration) []wword {
	var out []wword
	for _, s := range tr.Segments {
		for _, w := range s.Words {
			ws := dur(w.Start)
			we := dur(w.End)
			if we <= start || ws >= end {
				continue
			}
			text := strings.TrimSpace(w.Word)
			if text == "" {
				continue
			}
			if ws < start {
				ws = start
			}
			if we > end {
				we = end
			}
			// Event times are normalized to clip-local offsets because renderer
			// operates on per-clip subtitle files, not full-timeline subtitles.
			out = append(out, wword{Start: ws - start, End: we - start, Text: sanitizeASS(text)})
		}
	}
	return out
}

func collectSegmentText(tr types.Transcript, start, end time.Duration) string {
	var parts []string
	for _, s := range tr.Segments {
		ss := dur(s.Start)
		se := dur(s.End)
		if se <= start || ss >= end {
			continue
		}
		if strings.TrimSpace(s.Text) != "" {
			parts = append(parts, strings.TrimSpace(s.Text))
		}
	}
	return strings.Join(parts, " ")
}

func packWords(words []wword) []line {
	var out []line
	cur := line{Start: words[0].Start}
	// Hard budgets trade exact transcript grouping for consistently readable
	// subtitle chunks on vertical-video layouts.
	charBudget := 42
	wordBudget := 9
	curLen := 0
	for i, w := range words {
		wl := len([]rune(w.Text))
		nextLen := curLen
		if curLen > 0 {
			nextLen++
		}
		nextLen += wl
		if len(cur.Words) >= wordBudget || nextLen > charBudget {
			cur.End = cur.Words[len(cur.Words)-1].End
			out = append(out, cur)
			cur = line{Start: w.Start}
			curLen = 0
		}
		cur.Words = append(cur.Words, w)
		if curLen > 0 {
			curLen++
		}
		curLen += wl
		if i == len(words)-1 {
			cur.End = w.End
			out = append(out, cur)
		}
	}
	return out
}

func renderASSKaraoke(lines []line) string {
	var b strings.Builder
	b.WriteString(assHeader())
	b.WriteString("\n[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, ln := range lines {
		b.WriteString("Dialogue: 0,")
		b.WriteString(assTime(ln.Start))
		b.WriteString(",")
		b.WriteString(assTime(ln.End))
		b.WriteString(",TikTok,,0,0,0,,")
		for _, w := range ln.Words {
			durCS := int((w.End - w.Start) / (10 * time.Millisecond))
			if durCS < 1 {
				durCS = 1
			}
			b.WriteString(fmt.Sprintf("{\\k%d}%s ", durCS, w.Text))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderASSPlain(text string, dur time.Duration) string {
	var b strings.Builder
	b.WriteString(assHeader())
	b.WriteString("\n[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	b.WriteString("Dialogue: 0,0:00:00.00,")
	b.WriteString(assTime(dur))
	b.WriteString(",TikTok,,0,0,0,,")
	b.WriteString(sanitizeASS(text))
	b.WriteString("\n")
	return b.String()
}

func assHeader() string {
	return strings.TrimSpace(`
[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: TikTok, Inter, 78, &H00FFFFFF, &H00FFD200, &H00000000, &H64000000, 1,0,0,0,100,100,0,0,1,6,2,2, 80,80,85,1
`)
}

func assTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	hs := int(d / time.Hour)
	d -= time.Duration(hs) * time.Hour
	ms := int(d / time.Minute)
	d -= time.Duration(ms) * time.Minute
	s := int(d / time.Second)
	d -= time.Duration(s) * time.Second
	cs := int(d / (10 * time.Millisecond))
	return fmt.Sprintf("%d:%02d:%02d.%02d", hs, ms, s, cs)
}

func sanitizeASS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "{", "(")
	s = strings.ReplaceAll(s, "}", ")")
	return strings.TrimSpace(s)
}

func dur(sec float64) time.Duration { return time.Duration(sec * float64(time.Second)) }
