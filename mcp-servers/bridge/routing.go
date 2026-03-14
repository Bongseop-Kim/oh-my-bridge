package main

import (
	"errors"
	"fmt"
	"strings"
)

// resolveModel returns the model name, its definition, and whether Claude should handle directly.
// If modelOverride is set, it bypasses config route lookup.
func resolveModel(category, modelOverride string, c Config, clis map[string]bool) (modelName string, def ModelDef, skip bool, reason string, err error) {
	if modelOverride != "" {
		d, ok := c.Models[modelOverride]
		if !ok {
			return "", ModelDef{}, false, "", fmt.Errorf("model override %q not found in config", modelOverride)
		}
		if !clis[d.Command] {
			return modelOverride, d, true, reasonCLINotInstalled, nil
		}
		return modelOverride, d, false, "", nil
	}

	routeVal, ok := c.Routes[category]
	if !ok {
		if c.DefaultRoute != "" {
			routeVal = c.DefaultRoute
		} else {
			return "", ModelDef{}, false, "", fmt.Errorf("category %q not found in config routes", category)
		}
	}
	if routeVal == "claude" {
		return "claude", ModelDef{}, true, reasonRouteConfigured, nil
	}

	d, ok := c.Models[routeVal]
	if !ok {
		return "", ModelDef{}, false, "", fmt.Errorf("model %q (from route for category %q) not found in config models", routeVal, category)
	}
	if !clis[d.Command] {
		return routeVal, d, true, reasonCLINotInstalled, nil
	}
	return routeVal, d, false, "", nil
}

// resolveCategoryOverrides returns the effective reasoningEffort and promptAppend
// for a given category. Priority: per-call input > category_overrides > ModelDef default.
func resolveCategoryOverrides(category string, input delegateInput, modelDef ModelDef, overrides map[string]CategoryOverride) (reasoningEffort, promptAppend string) {
	reasoningEffort = modelDef.ReasoningEffort
	if co, ok := overrides[category]; ok {
		if co.ReasoningEffort != "" {
			reasoningEffort = co.ReasoningEffort
		}
		promptAppend = co.PromptAppend
	}
	if input.ReasoningEffort != "" {
		reasoningEffort = input.ReasoningEffort
	}
	return
}

func classifyCliError(err error) string {
	if errors.Is(err, ErrTimeout) {
		return reasonCLIErrorTimeout
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests") {
		return reasonCLIErrorRateLimit
	}
	return reasonCLIErrorCrash
}
