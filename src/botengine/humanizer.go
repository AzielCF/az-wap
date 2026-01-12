package botengine

import (
	"context"
	"math/rand"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Humanizer manages human-like behavior simulation for bot responses.
type Humanizer struct {
	Enabled bool
	rng     *rand.Rand
}

// TypingProfile defines the bot's typing style.
type TypingProfile struct {
	BaseCharDelayMs        int
	CharDelayVarianceMs    int
	PunctuationPauseChance int
	PunctuationPauseMinMs  int
	PunctuationPauseMaxMs  int
	WordsPerBreak          int
	WordsBreakVariance     int
	ThinkingPauseChance    int
	ThinkingPauseMinMs     int
	ThinkingPauseMaxMs     int
}

// DefaultProfile simulates an average human typer.
var DefaultProfile = TypingProfile{
	BaseCharDelayMs:        12,
	CharDelayVarianceMs:    8,
	PunctuationPauseChance: 40,
	PunctuationPauseMinMs:  150,
	PunctuationPauseMaxMs:  350,
	WordsPerBreak:          20,
	WordsBreakVariance:     12,
	ThinkingPauseChance:    25,
	ThinkingPauseMinMs:     200,
	ThinkingPauseMaxMs:     500,
}

// FastTyperProfile simulates a fast typer (e.g., experienced support agent).
var FastTyperProfile = TypingProfile{
	BaseCharDelayMs:        6,
	CharDelayVarianceMs:    4,
	PunctuationPauseChance: 20,
	PunctuationPauseMinMs:  80,
	PunctuationPauseMaxMs:  180,
	WordsPerBreak:          35,
	WordsBreakVariance:     15,
	ThinkingPauseChance:    10,
	ThinkingPauseMinMs:     100,
	ThinkingPauseMaxMs:     250,
}

// CasualTyperProfile simulates a relaxed typer, potentially on mobile.
var CasualTyperProfile = TypingProfile{
	BaseCharDelayMs:        18,
	CharDelayVarianceMs:    12,
	PunctuationPauseChance: 60,
	PunctuationPauseMinMs:  250,
	PunctuationPauseMaxMs:  600,
	WordsPerBreak:          12,
	WordsBreakVariance:     8,
	ThinkingPauseChance:    40,
	ThinkingPauseMinMs:     350,
	ThinkingPauseMaxMs:     800,
}

func NewHumanizer(enabled bool) *Humanizer {
	return &Humanizer{
		Enabled: enabled,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SimulateTyping simulates human typing behavior using the default profile.
func (h *Humanizer) SimulateTyping(ctx context.Context, t Transport, chatID string, text string) bool {
	return h.SimulateTypingWithProfile(ctx, t, chatID, text, DefaultProfile)
}

// SimulateTypingWithProfile simulates human typing using a custom profile.
func (h *Humanizer) SimulateTypingWithProfile(ctx context.Context, t Transport, chatID string, text string, profile TypingProfile) bool {
	if !h.Enabled || t == nil {
		return true
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}

	// 1. Initial delay (reading/thinking time)
	initialDelay := time.Duration(50+h.rng.Intn(100)) * time.Millisecond
	if !h.sleep(ctx, initialDelay) {
		return false
	}

	// 2. Start typing effect
	_ = t.SendPresence(ctx, chatID, true)
	defer h.stopTyping(ctx, t, chatID)

	// 3. Text analysis
	charCount := utf8.RuneCountInString(text)
	if charCount == 0 {
		charCount = len(text)
	}

	// Adjust base speed for short messages
	perCharBase := time.Duration(profile.BaseCharDelayMs) * time.Millisecond
	if charCount < 20 {
		perCharBase = time.Duration(profile.BaseCharDelayMs-4) * time.Millisecond
		if perCharBase < 4*time.Millisecond {
			perCharBase = 4 * time.Millisecond
		}
	}

	var (
		segmentChars      int
		wordCount         int
		lastWasSpace      = true
		lastWasNewline    = false
		consecutiveSpaces = 0
	)

	nextWordBreak := profile.WordsPerBreak + h.rng.Intn(profile.WordsBreakVariance*2) - profile.WordsBreakVariance
	if nextWordBreak < 5 {
		nextWordBreak = 5
	}

	flushSegment := func() bool {
		if segmentChars <= 0 {
			return true
		}
		variance := time.Duration(h.rng.Intn(profile.CharDelayVarianceMs+1)) * time.Millisecond
		perChar := perCharBase + variance
		delay := time.Duration(segmentChars) * perChar

		// Cap max delay per segment
		if delay > 4*time.Second {
			delay = 4 * time.Second
		}

		segmentChars = 0
		return h.sleep(ctx, delay)
	}

	pauseWithPresence := func(minMs, maxMs int) bool {
		ms := minMs
		if maxMs > minMs {
			ms = minMs + h.rng.Intn(maxMs-minMs+1)
		}

		if ms >= 200 {
			_ = t.SendPresence(ctx, chatID, false)
		}

		if !h.sleep(ctx, time.Duration(ms)*time.Millisecond) {
			return false
		}

		if ms >= 200 {
			_ = t.SendPresence(ctx, chatID, true)
		}
		return true
	}

	// 4. Character-by-character simulation
	runes := []rune(text)
	for i, r := range runes {
		segmentChars++

		isSpace := unicode.IsSpace(r)
		if isSpace {
			consecutiveSpaces++
			if !lastWasSpace {
				wordCount++
			}
			lastWasSpace = true
		} else {
			consecutiveSpaces = 0
			lastWasSpace = false
		}

		// Rule 1: Pause every N words (thinking pause)
		if wordCount >= nextWordBreak {
			perCharBase = time.Duration(profile.BaseCharDelayMs+h.rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond

			if !flushSegment() {
				return false
			}

			if h.rng.Intn(100) < profile.ThinkingPauseChance {
				if !pauseWithPresence(profile.ThinkingPauseMinMs, profile.ThinkingPauseMaxMs) {
					return false
				}
			}

			wordCount = 0
			nextWordBreak = profile.WordsPerBreak + h.rng.Intn(profile.WordsBreakVariance*2) - profile.WordsBreakVariance
			if nextWordBreak < 5 {
				nextWordBreak = 5
			}
			continue
		}

		// Rule 2: Strong punctuation pauses (. ! ?)
		if r == '.' || r == '!' || r == '?' {
			if i < len(runes)-1 && h.rng.Intn(100) < profile.PunctuationPauseChance {
				perCharBase = time.Duration(profile.BaseCharDelayMs+h.rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
				if !flushSegment() {
					return false
				}
				if !pauseWithPresence(profile.PunctuationPauseMinMs, profile.PunctuationPauseMaxMs) {
					return false
				}
			}
		}

		// Rule 3: Newline pauses
		if r == '\n' {
			perCharBase = time.Duration(profile.BaseCharDelayMs+h.rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
			if !flushSegment() {
				return false
			}

			if lastWasNewline {
				// Paragraph break
				if !pauseWithPresence(300, 700) {
					return false
				}
			} else {
				// Single line break
				if !pauseWithPresence(180, 450) {
					return false
				}
			}
			lastWasNewline = true
			continue
		}

		lastWasNewline = false

		// Rule 4: Micro-pauses for commas/colons
		if r == ',' || r == ':' || r == ';' {
			if h.rng.Intn(100) < 20 {
				if !flushSegment() {
					return false
				}
				if !h.sleep(ctx, time.Duration(60+h.rng.Intn(100))*time.Millisecond) {
					return false
				}
			}
		}

		// Rule 5: Emoji pauses (searching)
		if isEmoji(r) {
			if !flushSegment() {
				return false
			}
			if !h.sleep(ctx, time.Duration(100+h.rng.Intn(250))*time.Millisecond) {
				return false
			}
		}
	}

	// 5. Final flush and pre-send pause
	perCharBase = time.Duration(profile.BaseCharDelayMs+h.rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
	if !flushSegment() {
		return false
	}

	_ = t.SendPresence(ctx, chatID, false)
	return h.sleep(ctx, time.Duration(80+h.rng.Intn(180))*time.Millisecond)
}

func (h *Humanizer) stopTyping(ctx context.Context, t Transport, chatID string) {
	if t == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = t.SendPresence(stopCtx, chatID, false)
}

func (h *Humanizer) sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func isEmoji(r rune) bool {
	return (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
		(r >= 0x1F700 && r <= 0x1F77F) || // Alchemical Symbols
		(r >= 0x1F780 && r <= 0x1F7FF) || // Geometric Shapes Extended
		(r >= 0x1F800 && r <= 0x1F8FF) || // Supplemental Arrows-C
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols and Pictographs
		(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
		(r >= 0x1FA70 && r <= 0x1FAFF) || // Symbols and Pictographs Extended-A
		(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) || // Variation Selectors
		(r >= 0x1F1E0 && r <= 0x1F1FF) // Flags
}
