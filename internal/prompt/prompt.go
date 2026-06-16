package prompt

import (
	"fmt"
	"regexp"
)

var placeholderRE = regexp.MustCompile(`\{\{([A-Z0-9_]+)\}\}`)

// Substitute replaces all {{KEY}} placeholders in template with values from args.
// Returns an error if any placeholder has no corresponding arg, or any arg has
// no corresponding placeholder (catches typos in caller-supplied args).
//
// Keys must be uppercase ASCII letters, digits, or underscores ([A-Z0-9_]).
// Placeholders with lowercase or mixed-case keys (e.g. {{name}}) are not
// recognised and pass through unchanged — no error is returned for them.
func Substitute(template string, args map[string]string) (string, error) {
	// Collect all unique placeholder keys present in the template.
	matches := placeholderRE.FindAllStringSubmatch(template, -1)
	used := make(map[string]bool, len(matches))
	for _, m := range matches {
		used[m[1]] = true
	}

	// Every placeholder must have a matching arg.
	for key := range used {
		if _, ok := args[key]; !ok {
			return "", fmt.Errorf("template placeholder {{%s}} has no corresponding argument", key)
		}
	}

	// Every arg must have a matching placeholder.
	for key := range args {
		if !used[key] {
			return "", fmt.Errorf("argument %q has no corresponding {{%s}} placeholder in template", key, key)
		}
	}

	result := placeholderRE.ReplaceAllStringFunc(template, func(match string) string {
		key := match[2 : len(match)-2] // strip {{ and }}
		return args[key]
	})

	return result, nil
}
