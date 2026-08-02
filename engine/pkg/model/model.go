// Package model holds the shared, versioned data structures that flow through
// the GlassParse AI engine: the parser, extracted text, entities and processing
// results. Keeping these types in one package lets the cleaner, extractor,
// chunker, CLI and SDK agree on a single wire format.
package model

import "time"

// RenderMode describes how a page was fetched. The engine transparently swaps
// between a fast HTTP path and a headless-browser path (for SPA / React / Vue
// sites that require JavaScript execution).
type RenderMode string

const (
	// RenderHTTP means the HTML was fetched directly over the network.
	RenderHTTP RenderMode = "http"

	// RenderHeadless means a real browser rendered the page before it was read.
	RenderHeadless RenderMode = "headless"
)

// FetchOptions tunes a single page fetch: the anti-bot knobs (proxy, UA
// rotation, header spoofing) and the static-vs-headless decision.
type FetchOptions struct {
	URL         string        `json:"url"              yaml:"url"`
	UserAgent   string        `json:"user_agent"       yaml:"user_agent"`
	Proxy       string        `json:"proxy"            yaml:"proxy"`
	RotateUA    bool          `json:"rotate_ua"        yaml:"rotate_ua"`
	MaxRedirect int           `json:"max_redirect"     yaml:"max_redirect"`
	Timeout     time.Duration `json:"timeout"          yaml:"timeout"`
	Headless    bool          `json:"headless"         yaml:"headless"`
}

// KV is an ordered key/value pair used where header ordering matters.
type KV struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

// Page is the deliverable of the cleaner stage: clean markdown plus semantic
// HTML plus the facts the dashboard needs to render live telemetry.
type Page struct {
	URL        string        `json:"url"`
	StatusCode int           `json:"status_code"`
	Mode       RenderMode    `json:"mode"`
	Duration   time.Duration `json:"duration_ns"`
	FetchedAt  time.Time     `json:"fetched_at"`
	Meta       Metadata      `json:"metadata"`
	Markdown   string        `json:"markdown"`   // cleaned, optimised Markdown.
	CleanHTML  string        `json:"clean_html"` // semantic HTML retained for humans / re-processing.
	Stats      Stats         `json:"stats"`
}

// Metadata holds the <head>/<meta> level facts pipelines and dashboards want.
type Metadata struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Lang        string            `json:"lang"`
	Canonical   string            `json:"canonical"`
	SiteName    string            `json:"site_name"`
	Image       string            `json:"image"`
	Author      string            `json:"author"`
	Keywords    []string          `json:"keywords"`
	OG          map[string]string `json:"og"`
}

// Stats describes the measurable output of the cleaning pipeline — used for the
// live dashboard charts (throughput, data volume and density).
type Stats struct {
	InputBytes       int64   `json:"input_bytes"`
	OutputBytes      int64   `json:"output_bytes"`
	CompressionRatio float64 `json:"compression_ratio"` // output/input, lower is better
	RemovedNodes     int     `json:"removed_nodes"`
	Headings         int     `json:"headings"`
	Links            int     `json:"links"`
	Tables           int     `json:"tables"`
	Images           int     `json:"images"`
	NodesBefore      int     `json:"nodes_before"`
	NodesAfter       int     `json:"nodes_after"`
}

// Entity is one structured datum the extractor found (price, title, article,
// product card…). Confidence is the Self-Healing scorer's estimate in [0,1].
type Entity struct {
	Type       string            `json:"type"`                // "price","title","article","link","phone","email",…
	Label      string            `json:"label,omitempty"`
	Value      string            `json:"value"`
	Confidence float64           `json:"confidence"`
	Selector   string            `json:"selector,omitempty"` // the path that located it.
	Children   []*Entity         `json:"children,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	LineRef    int               `json:"-"` // internal reference, never serialised
}