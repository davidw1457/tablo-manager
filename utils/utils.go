package utils

func Substring(s string, start int, length int) string {
	sRunes := []rune(s)
	if start > len(sRunes)-1 {
		return ""
	} else if start < 0 {
		start += len(sRunes)
	}

	length = min(length, len(sRunes)-start)

	return string(sRunes[start : start+length])
}

func min(x int, y int) int {
	switch {
	case x < y:
		return x
	default:
		return y
	}
}

func SanitizeFileString(s string) string {
	var out []rune
	for _, v := range s {
		switch v {
		case '<':
		case '>':
		case ':':
		case '"':
		case '/':
		case '\\':
		case '|':
		case '?':
		case '*':
			out = append(out, '_')
		default:
			out = append(out, v)
		}
	}
	return string(out)
}
