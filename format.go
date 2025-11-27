package logger

import (
	"fmt"
	"net/url"
	"strings"
)

type color int

const (
	Blue color = iota
	Cyan
	Green
	Purple
	Red
	Yellow
	Gray
	Magenta
	BrightCyan
)

// Keep lowercase aliases for internal use
const (
	blue       = Blue
	cyan       = Cyan
	green      = Green
	purple     = Purple
	red        = Red
	yellow     = Yellow
	gray       = Gray
	magenta    = Magenta
	brightCyan = BrightCyan
)

// FormatString applies ANSI color codes to the given text
func FormatString(text string, c color, bold bool) string {
	return formatString(text, c, bold)
}

// formatString applies ANSI color codes to the given text (internal)
func formatString(text string, c color, bold bool) string {
	var colorCode string
	if bold {
		colorCode = "\033[1m"
	}

	colorReset := "\033[0m"

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
	case gray:
		colorCode += "\033[90m"
	case magenta:
		colorCode += "\033[95m" // Bright magenta
	case brightCyan:
		colorCode += "\033[96m" // Bright cyan
	default:
		colorCode += colorReset
	}

	return fmt.Sprintf("%s%s%s", colorCode, text, colorReset)
}

// getFullPath constructs the full path including query parameters
func getFullPath(u *url.URL) string {
	if u.RawQuery == "" {
		return u.Path
	}
	return fmt.Sprintf("%s?%s", u.Path, u.RawQuery)
}

// formatStatusCode returns the status code as a string with appropriate color formatting
func formatStatusCode(code int) (string, LogLevel) {
	var (
		statusCode string
		logLevel   = Info
	)

	switch code / 100 {
	case 2:
		statusCode = formatString(fmt.Sprintf("%d", code), green, false)
	case 3:
		statusCode = formatString(fmt.Sprintf("%d", code), blue, false)
	case 4, 5:
		statusCode = formatString(fmt.Sprintf("%d", code), red, false)
		logLevel = Error
	default:
		statusCode = fmt.Sprintf("%d", code)
	}

	return statusCode, logLevel
}

func isSensitiveKey(key string, redactKeys []string) bool {
	for _, k := range redactKeys {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func redactValueIfNeeded(key string, value any, cfg Config) any {
	if isSensitiveKey(key, cfg.RedactKeys) {
		return cfg.RedactMask
	}
	return value
}

// FormatStatusCode returns the formatted status code and the appropriate log level (exported for middleware)
func FormatStatusCode(statusCode int) (string, LogLevel) {
	return formatStatusCode(statusCode)
}

// GetFullPath builds the full URL path with query parameters (exported for middleware)
func GetFullPath(u *url.URL) string {
	return getFullPath(u)
}
