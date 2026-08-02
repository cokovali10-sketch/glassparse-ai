package fetcher

import (
	"context"
	"testing"

	"github.com/glassparse/engine/pkg/model"
)

func TestLooksSPA_DivApp(t *testing.T) {
	if !looksSPA([]byte(`<html><body><div id="app"></div><script src="/bundle.js"></script></body></html>`)) {
		t.Fatal("expected an empty #app mount to be flagged as SPA")
	}
}

func TestLooksSPA_StaticContent(t *testing.T) {
	html := []byte(`<html><body><main><h1>Title</h1><p>Real server-rendered content.</p></main></body></html>`)
	if looksSPA(html) {
		t.Fatal("server-rendered content should NOT be flagged as SPA")
	}
}

func TestRandomUAFromPool(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		seen[randomUA()] = true
	}
	if len(seen) == 0 {
		t.Fatal("randomUA never selected a UA")
	}
}

type fakeRenderer struct{ fired bool }

func (f *fakeRenderer) Fetch(_ context.Context, _ model.FetchOptions) ([]byte, int, error) {
	f.fired = true
	return []byte("<html><body>rendered</body></html>"), 200, nil
}