package logger

import "fmt"

type Color int

const (
	Blue Color = iota
	Cyan
	Green
	Purple
	Red
	Yellow
)

// Helper function to colorize strings
func FormatString(text string, color Color, bold bool) string {
	var colorCode string
	if bold {
		colorCode = "\033[1m"
	}

	reset := "\033[0m"

	switch color {
	case Blue:
		colorCode += "\033[34m"
	case Cyan:
		colorCode += "\033[36m"
	case Green:
		colorCode += "\033[032m"
	case Purple:
		colorCode += "\033[35m"
	case Red:
		colorCode += "\033[31m"
	case Yellow:
		colorCode += "\033[33m"
	default:
		colorCode += reset
	}

	return fmt.Sprintf("%s%s%s", colorCode, text, reset)
}
