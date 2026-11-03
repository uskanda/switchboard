package config

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// The kinds of a project's icon (Project.IconKind). Without an icon, the web inbox shows the forge's avatar.
const (
	IconURL   = "url"   // an image at an https:// URL, read without a token
	IconFile  = "file"  // an image file of this machine, an absolute path once loaded
	IconEmoji = "emoji" // one emoji (one grapheme), shown as text
)

// IconKind is the kind of p.Icon, "" when it is not set.
func (p Project) IconKind() string {
	k, _ := iconKind(p.Icon)
	return k
}

var errIcon = errors.New("must be an https:// URL of an image, the path of an image file starting with / or ~/, or one emoji")

func iconKind(icon string) (string, error) {
	switch {
	case icon == "":
		return "", nil
	case strings.HasPrefix(icon, "https://"):
		if len(icon) == len("https://") {
			return "", errIcon
		}
		return IconURL, nil
	case strings.HasPrefix(icon, "/"):
		return IconFile, nil
	case oneGrapheme(icon):
		return IconEmoji, nil
	}
	return "", errIcon
}

// oneGrapheme reports whether s is one user-perceived character. It knows the sequences emoji are written with — a
// variation selector, a skin tone, a ZWJ joining another character, a pair of regional indicators (a flag), a keycap,
// tag characters — and combining marks, not every rule of Unicode's segmentation.
func oneGrapheme(s string) bool {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || isExtend(r) {
		return false
	}
	rest := s[n:]
	if isRegional(r) {
		if r2, n2 := utf8.DecodeRuneInString(rest); isRegional(r2) {
			rest = rest[n2:]
		}
	}
	for rest != "" {
		r, n := utf8.DecodeRuneInString(rest)
		rest = rest[n:]
		switch {
		case r == 0x200D: // ZWJ: the next character joins this one
			if rest == "" {
				return false
			}
			_, n := utf8.DecodeRuneInString(rest)
			rest = rest[n:]
		case isExtend(r):
		default:
			return false
		}
	}
	return true
}

// isExtend is a character that continues the one before it.
func isExtend(r rune) bool {
	switch {
	case r >= 0xFE00 && r <= 0xFE0F: // variation selectors
	case r >= 0x1F3FB && r <= 0x1F3FF: // skin tones
	case r == 0x20E3: // combining enclosing keycap
	case r >= 0xE0020 && r <= 0xE007F: // tags
	case r >= 0x0300 && r <= 0x036F: // combining diacritical marks
	case r == 0x3099 || r == 0x309A: // combining kana voiced marks
	default:
		return false
	}
	return true
}

func isRegional(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }
