package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/AzielCF/az-wap/botengine/domain"
)

// Interpreter gestiona el proceso de enriquecimiento del input del usuario
type Interpreter struct {
	provider domain.MultimodalInterpreter
	apiKey   string
}

func NewInterpreter(p domain.MultimodalInterpreter, apiKey string) *Interpreter {
	return &Interpreter{provider: p, apiKey: apiKey}
}

// EnrichInput toma el texto original y los medios, y devuelve un texto enriquecido con descripciones y el costo asociado
func (i *Interpreter) EnrichInput(ctx context.Context, model string, input domain.BotInput) (string, *domain.UsageStats, error) {
	if i == nil {
		return input.Text, nil, nil
	}
	if len(input.Medias) == 0 {
		return input.Text, nil, nil
	}

	// Map for quick friendly name lookup by path
	friendlyNames := make(map[string]string)
	if resList, ok := input.Metadata["session_resources"].([]map[string]string); ok {
		for _, res := range resList {
			if path, ok := res["path"]; ok {
				friendlyNames[path] = res["name"]
			}
		}
	}

	var toAnalyze []*domain.BotMedia
	var resourceNotes []string

	processed := make(map[string]bool)

	for _, m := range input.Medias {
		if m == nil || processed[m.LocalPath] {
			continue
		}
		processed[m.LocalPath] = true

		displayName := m.FileName
		if fn, ok := friendlyNames[m.LocalPath]; ok && fn != "" {
			displayName = fn
		}

		if m.State == domain.MediaStateAnalyzed {
			toAnalyze = append(toAnalyze, m)
		} else {
			stateLabel := "AVAILABLE"
			if m.State == domain.MediaStateBlocked {
				stateLabel = "BLOCKED"
			}
			resourceNotes = append(resourceNotes, fmt.Sprintf("[RESOURCE %s: %s]", stateLabel, displayName))
		}
	}

	if len(toAnalyze) > 0 && i.provider != nil {
		res, usage, err := i.provider.Interpret(ctx, i.apiKey, model, input.Text, input.Language, toAnalyze)
		if err != nil {
			return input.Text, nil, err
		}

		var contextParts []string
		contextParts = append(contextParts, input.Text)

		for j, t := range res.Transcriptions {
			contextParts = append(contextParts, fmt.Sprintf("[Audio %d]: %s", j+1, t))
		}
		for j, d := range res.Descriptions {
			contextParts = append(contextParts, fmt.Sprintf("[Image %d]: %s", j+1, d))
		}
		for j, s := range res.Summaries {
			contextParts = append(contextParts, fmt.Sprintf("[Document %d]: %s", j+1, s))
		}
		for j, v := range res.VideoSummaries {
			contextParts = append(contextParts, fmt.Sprintf("[Video %d]: %s", j+1, v))
		}

		if len(resourceNotes) > 0 {
			contextParts = append(contextParts, resourceNotes...)
		}

		return strings.Join(contextParts, "\n\n"), usage, nil
	} else if len(resourceNotes) > 0 {
		return input.Text + "\n\n" + strings.Join(resourceNotes, "\n"), nil, nil
	}

	return input.Text, nil, nil
}
