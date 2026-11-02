package agent

import (
	"fmt"
	"strings"
)

// stopReason says why the runner ended a task before the model finished it.
type stopReason int

const (
	stopRounds stopReason = iota
	stopCalls
	stopTimeout
)

// stopTexts are produced by the runner itself (not the model), so they are
// translated here. Keys are the languages the panel ships; anything else uses English.
var stopTexts = map[string]map[stopReason]string{
	"en": {
		stopRounds:  "I stopped this task because it needed too many steps (limit: %d rounds).",
		stopCalls:   "I stopped this task because it needed too many operations (limit: %d).",
		stopTimeout: "I stopped this task because it took too long.",
	},
	"es": {
		stopRounds:  "He detenido esta tarea porque necesitaba demasiados pasos (límite: %d rondas).",
		stopCalls:   "He detenido esta tarea porque necesitaba demasiadas operaciones (límite: %d).",
		stopTimeout: "He detenido esta tarea porque estaba tardando demasiado.",
	},
}

var doneTexts = map[string]struct{ done, nothing string }{
	"en": {done: "Done so far:", nothing: "Nothing was changed."},
	"es": {done: "Hecho hasta ahora:", nothing: "No se ha cambiado nada."},
}

// languageKey maps a session language ("es", "es-ES", "en"…) to a key of the maps above.
func languageKey(language string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "es") {
		return "es"
	}
	return "en"
}

// stopReply explains why the task was stopped and lists what was done in this run.
func stopReply(language string, reason stopReason, limits Limits, steps []Step) string {
	lang := languageKey(language)
	text := stopTexts[lang][reason]
	switch reason {
	case stopRounds:
		text = fmt.Sprintf(text, limits.MaxRounds)
	case stopCalls:
		text = fmt.Sprintf(text, limits.MaxCalls)
	}
	var b strings.Builder
	b.WriteString(text)
	done := 0
	for _, s := range steps {
		if s.Status != StatusDone {
			continue
		}
		if done == 0 {
			b.WriteString("\n\n")
			b.WriteString(doneTexts[lang].done)
		}
		done++
		b.WriteString("\n- ")
		b.WriteString(s.Summary)
	}
	if done == 0 {
		b.WriteString(" ")
		b.WriteString(doneTexts[lang].nothing)
	}
	return b.String()
}
