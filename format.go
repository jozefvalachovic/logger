package logger

import "fmt"

// formatString is now unexported (internal use only)
func formatString(text string, c color, bold bool) string {
	var colorCode string
	if bold {
		colorCode = "\033[1m"
	}

	reset := "\033[0m"

	switch c {
	case blue:
		colorCode += "\033[34m"
	case cyan:
		colorCode += "\033[36m"
	case green:
		colorCode += "\033[032m"
	case purple:
		colorCode += "\033[35m"
	case red:
		colorCode += "\033[31m"
	case yellow:
		colorCode += "\033[33m"
	default:
		colorCode += reset
	}

	return fmt.Sprintf("%s%s%s", colorCode, text, reset)
}
