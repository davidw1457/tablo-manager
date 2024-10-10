package stringmanip

import (
	"regexp"
)

func Substring(s string, start int, length int) string {
	sRunes := []rune(s)
	if start > len(sRunes)-1 {
		return ""
	} else if start < 0 {
		start += len(sRunes)
	}

	length = minInt(length, len(sRunes)-start)

	return string(sRunes[start : start+length])
}

func SanitizeFile(s string) string {
	illegalChars := regexp.MustCompile(`[<>:"\/\\|?*]`)
	out := illegalChars.ReplaceAllString(s, "_")
	return out
}

func SanitizeSql(s string) string {
	illegalChars := regexp.MustCompile(`'`)
	out := illegalChars.ReplaceAllString(s, `''`)
	return out
}

func minInt(x int, y int) int {
	switch {
	case x < y:
		return x
	default:
		return y
	}
}

