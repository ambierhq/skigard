package core

import (
	"strconv"

	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// ConfigKeys are the settings `config get|set` manages.
var ConfigKeys = []string{"mode", "threshold", "auto_review"}

// ConfigGet returns the effective value of a setting, as text.
func (a *App) ConfigGet(key string) (string, error) {
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return "", err
	}
	switch key {
	case "mode":
		return man.Settings.EffectiveMode(), nil
	case "threshold":
		return strconv.Itoa(man.Settings.EffectiveThreshold()), nil
	case "auto_review":
		return strconv.FormatBool(man.Settings.AutoReview), nil
	}
	return "", util.Errf("unknown setting %q (want mode, threshold or auto_review)", util.Clean(key))
}

// ConfigSet changes a setting. Changing mode or threshold decides how much an
// agent may install on its own, so it needs a human: it is refused inside an
// agent and requires confirm (the terminal prompt) to succeed.
func (a *App) ConfigSet(key, value string, confirm Confirmer) (old string, err error) {
	unlock, err := a.Home.Lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	if err := a.Home.Init(); err != nil {
		return "", err
	}
	man, err := store.LoadManifest(a.Home)
	if err != nil {
		return "", err
	}
	s := &man.Settings
	human := false
	switch key {
	case "mode":
		if value != store.ModeStrict && value != store.ModeAuto {
			return "", util.Errf("mode must be strict or auto")
		}
		old, human = s.EffectiveMode(), true
	case "threshold":
		n, perr := strconv.Atoi(value)
		if perr != nil || n < 1 || n > 10 {
			return "", util.Errf("threshold must be a number from 1 to 10")
		}
		old, human = strconv.Itoa(s.EffectiveThreshold()), true
	case "auto_review":
		b, perr := strconv.ParseBool(value)
		if perr != nil {
			return "", util.Errf("auto_review must be true or false")
		}
		old = strconv.FormatBool(s.AutoReview)
		value = strconv.FormatBool(b)
	default:
		return "", util.Errf("unknown setting %q (want mode, threshold or auto_review)", util.Clean(key))
	}
	if human {
		if v, ok := a.agentVar(); ok {
			return "", util.Policyf("refusing to change %s: this looks like it is running inside an agent harness (%s). Changing the mode or threshold is a human step: run `skigard config set %s %s` in a normal terminal", key, v, key, value)
		}
		if confirm == nil {
			return "", util.Errf("config set %s needs an interactive confirmation", key)
		}
		if err := confirm("Change setting "+key+": "+old+" -> "+value+"\n(this decides what `skigard add` may install without asking you)", key); err != nil {
			return "", err
		}
	}
	switch key {
	case "mode":
		s.Mode = value
	case "threshold":
		s.Threshold, _ = strconv.Atoi(value)
	case "auto_review":
		s.AutoReview = value == "true"
	}
	if err := s.Validate(); err != nil {
		return "", util.Errf("%v", err)
	}
	return old, store.SaveManifest(a.Home, man)
}
