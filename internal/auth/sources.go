package auth

import (
	"fmt"
	"sort"
	"strings"
)

// BrowserInfo describes a supported browser for help text and completion.
type BrowserInfo struct {
	Name            string
	Summary         string
	SupportsProfile bool
}

type browserSource struct {
	name            string
	summary         string
	supportsProfile bool
	extract         func(profile string) (*Session, error)
}

// browserSources is the registry of importable browsers, in display order.
var browserSources = []browserSource{
	chromiumBrowser("chrome", "Google Chrome cookie store on disk", chromiumPaths{
		darwin: "Google/Chrome", linux: ".config/google-chrome", windows: "Google/Chrome/User Data",
		queries: chromiumSafeStorage("Chrome Safe Storage"),
	}),
	chromiumBrowser("brave", "Brave cookie store on disk", chromiumPaths{
		darwin: "BraveSoftware/Brave-Browser", linux: ".config/BraveSoftware/Brave-Browser",
		windows: "BraveSoftware/Brave-Browser/User Data",
		queries: chromiumSafeStorage("Brave Safe Storage", "Brave Browser Safe Storage"),
	}),
	chromiumBrowser("edge", "Microsoft Edge cookie store on disk", chromiumPaths{
		darwin: "Microsoft Edge", linux: ".config/microsoft-edge", windows: "Microsoft/Edge/User Data",
		queries: chromiumSafeStorage("Microsoft Edge Safe Storage"),
	}),
	chromiumBrowser("arc", "Arc cookie store on disk", chromiumPaths{
		darwin: "Arc/User Data", linux: ".config/arc", windows: "Arc/User Data",
		queries: chromiumSafeStorage("Arc Safe Storage"),
	}),
	chromiumBrowser("chromium", "Chromium cookie store on disk", chromiumPaths{
		darwin: "Chromium", linux: ".config/chromium", windows: "Chromium/User Data",
		queries: chromiumSafeStorage("Chromium Safe Storage"),
	}),
	geckoBrowser("firefox", "Firefox profile on disk (browser need not be running)", "Firefox", ".mozilla/firefox", "Mozilla/Firefox/Profiles"),
	geckoBrowser("zen", "Zen Browser profile on disk (Firefox-based)", "zen", ".zen", "zen/Profiles"),
	{
		name:    "safari",
		summary: "Safari cookie store (macOS; needs Full Disk Access)",
		extract: func(string) (*Session, error) { return extractSafari() },
	},
}

func chromiumBrowser(name, summary string, p chromiumPaths) browserSource {
	return browserSource{
		name:    name,
		summary: summary,
		extract: func(string) (*Session, error) { return extractChromium(p) },
	}
}

func geckoBrowser(name, summary, darwin, linux, windows string) browserSource {
	return browserSource{
		name:            name,
		summary:         summary,
		supportsProfile: true,
		extract: func(profile string) (*Session, error) {
			base, err := geckoBaseDir(darwin, linux, windows)
			if err != nil {
				return nil, err
			}
			return extractGecko(base, profile)
		},
	}
}

// SupportedBrowsers returns registry metadata for help text and completions.
func SupportedBrowsers() []BrowserInfo {
	out := make([]BrowserInfo, 0, len(browserSources))
	for _, s := range browserSources {
		out = append(out, BrowserInfo{Name: s.name, Summary: s.summary, SupportsProfile: s.supportsProfile})
	}
	return out
}

// ImportBrowser extracts a Notion session from the named browser. profile
// selects a Firefox-family profile and is ignored by other browsers.
func ImportBrowser(name, profile string) (*Session, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, s := range browserSources {
		if s.name == want {
			return s.extract(profile)
		}
	}
	return nil, fmt.Errorf("unknown browser %q (supported: %s)", name, strings.Join(browserNames(), ", "))
}

func browserNames() []string {
	names := make([]string, 0, len(browserSources))
	for _, s := range browserSources {
		names = append(names, s.name)
	}
	sort.Strings(names)
	return names
}
