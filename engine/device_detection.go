package engine

import "strings"

func mobileBrowser(userAgent, platform string, touchPoints int, mobile bool) bool {
	if mobile {
		return true
	}
	userAgent = strings.ToLower(userAgent)
	for _, token := range []string{"android", "iphone", "ipad", "ipod", "mobile", "tablet"} {
		if strings.Contains(userAgent, token) {
			return true
		}
	}
	return touchPoints > 1 && (platform == "MacIntel" || strings.Contains(userAgent, "macintosh"))
}
