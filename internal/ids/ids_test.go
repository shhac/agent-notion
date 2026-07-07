package ids

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"30a61d9c1112802f95fef30d3a601ec5", "30a61d9c-1112-802f-95fe-f30d3a601ec5"},
		{"30a61d9c-1112-802f-95fe-f30d3a601ec5", "30a61d9c-1112-802f-95fe-f30d3a601ec5"},
		{"30A61D9C1112802F95FEF30D3A601EC5", "30a61d9c-1112-802f-95fe-f30d3a601ec5"},
		{"not-a-uuid", "not-a-uuid"},
		{"", ""},
		{"12345", "12345"},
		{"30a61d9c1112802f95fef30d3a601ecZ", "30a61d9c1112802f95fef30d3a601ecZ"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
