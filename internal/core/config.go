package core

import (
	"strconv"

	"github.com/ambierhq/skigard/internal/review"
	"github.com/ambierhq/skigard/internal/store"
	"github.com/ambierhq/skigard/internal/util"
)

// ConfigKeys are the settings `config get|set` manages.
var ConfigKeys = []string{"mode", "threshold", "auto_review", "review_harness", "marketplace"}

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
		return strconv.FormatBool(man.Settings.EffectiveAutoReview()), nil
	case "review_harness":
		return a.DefaultReviewHarness(man.Settings), nil
	case "marketplace":
		return strconv.FormatBool(man.Settings.EffectiveMarketplace()), nil
	}
	return "", util.Errf("unknown setting %q (want mode, threshold, auto_review, review_harness or marketplace)", util.Clean(key))
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
		old = strconv.FormatBool(s.EffectiveAutoReview())
		value = strconv.FormatBool(b)
	case "marketplace":
		b, perr := strconv.ParseBool(value)
		if perr != nil {
			return "", util.Errf("marketplace must be true or false")
		}
		old = strconv.FormatBool(s.EffectiveMarketplace())
		value = strconv.FormatBool(b)
	case "review_harness":
		// "auto" clears the saved choice so the first detected reviewer is used again.
		old = a.DefaultReviewHarness(*s)
		if value != "auto" {
			if _, ok := review.TemplateFor(value); !ok {
				return "", util.Errf("review_harness must be auto or a harness that can review (see `skigard review --list`)")
			}
		}
	default:
		return "", util.Errf("unknown setting %q (want mode, threshold, auto_review, review_harness or marketplace)", util.Clean(key))
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
	case "review_harness":
		s.ReviewHarness = value
		if value == "auto" {
			s.ReviewHarness = ""
		}
	case "auto_review":
		on := value == "true"
		s.AutoReview = &on
	case "marketplace":
		on := value == "true"
		s.Marketplace = &on
	}
	if err := s.Validate(); err != nil {
		return "", util.Errf("%v", err)
	}
	if err := store.SaveManifest(a.Home, man); err != nil {
		return "", err
	}
	if key == "marketplace" && value == "true" {
		// Build now so verify passes right away. The setting is saved either way; a failed build is reported.
		if err := a.marketplaceBuild(); err != nil {
			return old, util.Errf("marketplace is now enabled, but the build failed: %v; fix it and run: skigard marketplace build", err)
		}
	}
	return old, nil
}
