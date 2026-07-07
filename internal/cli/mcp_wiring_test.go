package cli

import (
	"testing"

	agentmcp "github.com/shhac/lib-agent-mcp"
)

// TestMCPWiring pins the tool-surface curation: data groups exposed,
// read-only groups annotated, credential/config groups kept out.
func TestMCPWiring(t *testing.T) {
	root := newRoot("test")

	byName := map[string]annotationSet{}
	for _, c := range root.Commands() {
		byName[c.Name()] = annotationSet{
			expose:   c.Annotations[agentmcp.AnnotationExpose] == "true",
			readOnly: c.Annotations[agentmcp.AnnotationReadOnly] == "true",
			skip:     c.Annotations[agentmcp.AnnotationSkip] == "true",
		}
	}

	if _, ok := byName["mcp"]; !ok {
		t.Fatal("mcp command not registered")
	}
	for _, name := range []string{"search", "page", "block", "database", "comment", "user", "export", "activity", "ai"} {
		if !byName[name].expose {
			t.Errorf("%s should be exposed", name)
		}
	}
	for _, name := range []string{"search", "user", "activity"} {
		if !byName[name].readOnly {
			t.Errorf("%s should be read-only", name)
		}
	}
	for _, name := range []string{"auth", "config"} {
		if byName[name].expose {
			t.Errorf("%s must not be exposed", name)
		}
		if !byName[name].skip {
			t.Errorf("%s should be skipped", name)
		}
	}
}

type annotationSet struct {
	expose, readOnly, skip bool
}
