package infrastructure

import (
	"context"
	"math/rand"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/AzielCF/az-wap/botengine/domain"
)

// Humanizer manages human-like behavior simulation for bot responses.
type Humanizer struct {
	Enabled bool
	Rng     *rand.Rand

	// Configuration
	CharReadingSpeedMs      int
	MaxReadingTime          time.Duration
	BaseQuoteChance         int
	DelayedQuoteChance      int
	MultiBubbleQuoteChance  int
	AdaptiveDebouncePercent int
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
		Enabled:                 enabled,
		Rng:                     rand.New(rand.NewSource(time.Now().UnixNano())),
		CharReadingSpeedMs:      25,
		MaxReadingTime:          6 * time.Second,
		BaseQuoteChance:         25,
		DelayedQuoteChance:      90,
		MultiBubbleQuoteChance:  100,
		AdaptiveDebouncePercent: 50,
	}
}

// CalculateReadingTime computes how long it takes to read a given text.
func (h *Humanizer) CalculateReadingTime(text string) time.Duration {
	t := time.Duration(len(text)) * time.Duration(h.CharReadingSpeedMs) * time.Millisecond
	if t > h.MaxReadingTime {
		return h.MaxReadingTime
	}
	return t
}

// GetDebounceDuration returns the grouping time adjusted by variance and user activity.
func (h *Humanizer) GetDebounceDuration(base time.Duration, msgLen int, textCount int) time.Duration {
	// 0. Sticky Wait (Patience)
	// If message is relatively short (common sentence), wait significantly for media or follow-up.
	if msgLen < 50 {
		return base + 5000*time.Millisecond
	}

	// 1. Reading Time
	reading := h.CalculateReadingTime(string(make([]byte, msgLen))) // dummy string length
	if reading > base {
		base = reading
	}

	// 2. Universal Padding (Always wait a bit more than reading time)
	// This covers the time it takes to "reach for the record button"
	base = base + 2000*time.Millisecond

	// 3. Adaptive Economy (if user is spamming)
	if textCount > 3 {
		base = base + (base * time.Duration(h.AdaptiveDebouncePercent) / 100)
	}

	// 4. Human Variance (85% - 115%)
	variance := time.Duration(h.Rng.Intn(30)-15) * (base / 100)
	return base + variance
}

// SimulateTyping simulates human typing behavior using the default profile.
func (h *Humanizer) SimulateTyping(ctx context.Context, t domain.Transport, chatID string, text string) bool {
	return h.SimulateTypingWithProfile(ctx, t, chatID, text, DefaultProfile)
}

// SimulateTypingWithProfile simulates human typing using a custom profile.
func (h *Humanizer) SimulateTypingWithProfile(ctx context.Context, t domain.Transport, chatID string, text string, profile TypingProfile) bool {
	if !h.Enabled || t == nil {
		return true
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}

	// 1. Initial delay (reading/thinking time)
	initialDelay := time.Duration(50+h.Rng.Intn(100)) * time.Millisecond
	if !h.sleep(ctx, initialDelay) {
		return false
	}

	// 2. Start typing effect
	_ = t.SendPresence(ctx, chatID, true, false)
	defer h.stopTyping(t, chatID)

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

	nextWordBreak := profile.WordsPerBreak + h.Rng.Intn(profile.WordsBreakVariance*2) - profile.WordsBreakVariance
	if nextWordBreak < 5 {
		nextWordBreak = 5
	}

	flushSegment := func() bool {
		if segmentChars <= 0 {
			return true
		}
		variance := time.Duration(h.Rng.Intn(profile.CharDelayVarianceMs+1)) * time.Millisecond
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
			ms = minMs + h.Rng.Intn(maxMs-minMs+1)
		}

		if ms >= 200 {
			_ = t.SendPresence(ctx, chatID, false, false)
		}

		if !h.sleep(ctx, time.Duration(ms)*time.Millisecond) {
			return false
		}

		if ms >= 200 {
			_ = t.SendPresence(ctx, chatID, true, false)
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
			perCharBase = time.Duration(profile.BaseCharDelayMs+h.Rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond

			if !flushSegment() {
				return false
			}

			if h.Rng.Intn(100) < profile.ThinkingPauseChance {
				if !pauseWithPresence(profile.ThinkingPauseMinMs, profile.ThinkingPauseMaxMs) {
					return false
				}
			}

			wordCount = 0
			nextWordBreak = profile.WordsPerBreak + h.Rng.Intn(profile.WordsBreakVariance*2) - profile.WordsBreakVariance
			if nextWordBreak < 5 {
				nextWordBreak = 5
			}
			continue
		}

		// Rule 2: Strong punctuation pauses (. ! ?)
		if r == '.' || r == '!' || r == '?' {
			if i < len(runes)-1 && h.Rng.Intn(100) < profile.PunctuationPauseChance {
				perCharBase = time.Duration(profile.BaseCharDelayMs+h.Rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
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
			perCharBase = time.Duration(profile.BaseCharDelayMs+h.Rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
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
			if h.Rng.Intn(100) < 20 {
				if !flushSegment() {
					return false
				}
				if !h.sleep(ctx, time.Duration(60+h.Rng.Intn(100))*time.Millisecond) {
					return false
				}
			}
		}

		// Rule 5: Emoji pauses (searching)
		if isEmoji(r) {
			if !flushSegment() {
				return false
			}
			if !h.sleep(ctx, time.Duration(100+h.Rng.Intn(250))*time.Millisecond) {
				return false
			}
		}
	}

	// 5. Final flush and pre-send pause
	perCharBase = time.Duration(profile.BaseCharDelayMs+h.Rng.Intn(profile.CharDelayVarianceMs)) * time.Millisecond
	if !flushSegment() {
		return false
	}

	_ = t.SendPresence(ctx, chatID, false, false)
	return h.sleep(ctx, time.Duration(80+h.Rng.Intn(180))*time.Millisecond)
}

func (h *Humanizer) stopTyping(t domain.Transport, chatID string) {
	if t == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = t.SendPresence(stopCtx, chatID, false, false)
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

// SplitIntoBubbles breaks a long text into logically separated chunks (paragraphs)
// to simulate multiple "message bubbles" like a human would.
func (h *Humanizer) SplitIntoBubbles(text string) []string {
	if text == "" {
		return nil
	}

	// ANTI-BAN & REALISM: 30% chance to NOT split at all, even if it has paragraphs.
	if h.Rng.Intn(100) < 30 {
		return []string{text}
	}

	// 1. Separate by double newlines (paragraphs)
	paragraphs := strings.Split(text, "\n\n")
	var bubbles []string

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// 2. If a paragraph is still too long (e.g. > 600 chars), split by sentences
		if len(p) > 600 {
			sentences := h.splitIntoSentences(p)
			bubbles = append(bubbles, sentences...)
		} else {
			bubbles = append(bubbles, p)
		}
	}

	// SAFETY LIMIT: Max 3 bubbles to avoid ban risk on non-official API
	if len(bubbles) <= 3 {
		return bubbles
	}

	// If we have more than 3, we take the first 2 as is,
	// and merge ALL remaining text into the 3rd bubble.
	result := []string{bubbles[0], bubbles[1]}
	remaining := strings.Join(bubbles[2:], "\n\n")
	result = append(result, remaining)

	return result
}

func (h *Humanizer) splitIntoSentences(text string) []string {
	var sentences []string
	current := ""

	// Basic sentence splitter by . ! ? followed by space or newline
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current += string(runes[i])
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			if i+1 < len(runes) && (unicode.IsSpace(runes[i+1]) || runes[i+1] == '\n') {
				sentences = append(sentences, strings.TrimSpace(current))
				current = ""
			}
		}
	}

	if strings.TrimSpace(current) != "" {
		sentences = append(sentences, strings.TrimSpace(current))
	}

	return sentences
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
