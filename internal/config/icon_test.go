package config

import (
	"strconv"
	"strings"
	"testing"
)

func withIcon(icon string) string {
	return strings.Replace(valid, "    path: ~/repos/sandbox\n", "    path: ~/repos/sandbox\n    icon: "+strconv.Quote(icon)+"\n", 1)
}

func TestIconKinds(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	for icon, want := range map[string]struct{ kind, value string }{
		"https://example.com/a.png": {IconURL, "https://example.com/a.png"},
		"/srv/icons/a.png":          {IconFile, "/srv/icons/a.png"},
		"~/icons/a.png":             {IconFile, "/home/alice/icons/a.png"},
		"🚀":                         {IconEmoji, "🚀"},
		"❤️":                        {IconEmoji, "❤️"},  // a variation selector
		"👍🏽":                        {IconEmoji, "👍🏽"},  // a skin tone
		"👩‍💻":                       {IconEmoji, "👩‍💻"}, // a ZWJ sequence
		"🇯🇵":                        {IconEmoji, "🇯🇵"},  // a flag
		"1️⃣":                       {IconEmoji, "1️⃣"}, // a keycap
		"S":                         {IconEmoji, "S"},
	} {
		c, err := Load(write(t, withIcon(icon)))
		if err != nil {
			t.Errorf("%q: %v", icon, err)
			continue
		}
		p := c.Projects[0]
		if p.IconKind() != want.kind || p.Icon != want.value {
			t.Errorf("%q: kind %q, icon %q; want %q, %q", icon, p.IconKind(), p.Icon, want.kind, want.value)
		}
	}
}

func TestNoIcon(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if k := c.Projects[0].IconKind(); k != "" {
		t.Fatalf("kind = %q, want none", k)
	}
}

func TestIconOfAnotherShapeIsRefused(t *testing.T) {
	for _, icon := range []string{"icon.png", "http://example.com/a.png", "🚀🚀", "ab", "https://"} {
		_, err := Load(write(t, withIcon(icon)))
		if err == nil || !strings.Contains(err.Error(), "projects[0].icon") {
			t.Errorf("%q: err = %v, want projects[0].icon reported", icon, err)
		}
	}
}
