package utils

import "strings"

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

func SanitizeFileString(s string) string {
	out := strings.ReplaceAll(s, "<", "_")
	out = strings.ReplaceAll(out, ">", "_")
	out = strings.ReplaceAll(out, ":", "_")
	out = strings.ReplaceAll(out, "\"", "_")
	out = strings.ReplaceAll(out, "/", "_")
	out = strings.ReplaceAll(out, "\\", "_")
	out = strings.ReplaceAll(out, "|", "_")
	out = strings.ReplaceAll(out, "?", "_")
	out = strings.ReplaceAll(out, "*", "_")

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
