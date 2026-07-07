package auth

import browsercookies "github.com/shhac/lib-agent-browsercookies"

// BrowserInfo describes a supported browser for help text and completion.
type BrowserInfo struct {
	Name            string
	Summary         string
	SupportsProfile bool
}

// SupportedBrowsers returns registry metadata for help text and completions,
// sourced from the shared cookie library.
func SupportedBrowsers() []BrowserInfo {
	sources := browsercookies.Sources()
	out := make([]BrowserInfo, 0, len(sources))
	for _, s := range sources {
		out = append(out, BrowserInfo{
			Name:            s.Name,
			Summary:         s.Summary,
			SupportsProfile: s.SupportsProfile,
		})
	}
	return out
}

// ImportBrowser extracts a Notion session from the named browser. profile
// selects a Firefox-family profile and is ignored by other browsers.
func ImportBrowser(name, profile string) (*Session, error) {
	return importBrowser(name, profile)
}

// importBrowser is the testable core: extra options (a fake Platform) let tests
// drive extraction against a fixture store on any host.
func importBrowser(name, profile string, opts ...browsercookies.Option) (*Session, error) {
	opts = append([]browsercookies.Option{browsercookies.WithProfile(profile)}, opts...)
	res, err := browsercookies.Extract(name, notionTarget, opts...)
	if err != nil {
		return nil, err
	}
	return &Session{TokenV2: res.Value, Source: res.Source}, nil
}
