package helper

import (
	"regexp"
	"strings"
)

func ToKebabCase(input string) string {
	// Use a regular expression to insert hyphens before each uppercase letter
	re := regexp.MustCompile("([a-z])([A-Z])")
	kebab := re.ReplaceAllString(input, "$1-$2")

	return strings.ToLower(kebab)
}
