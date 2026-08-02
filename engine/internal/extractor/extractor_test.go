package extractor

import "testing"

const sample = `<!DOCTYPE html><html><head><title>Acme</title></head><body>
<main>
  <h1>Acme Product 3000</h1>
  <p>The flagship costs about $12,499.00 and the entry tier is 9.99&euro;.</p>
  <p>Reach out at support@acme.io, or call +1 (212) 555-0173 for bulk.</p>
  <table><tr><th>A</th><th>B</th></tr><tr><td>x</td><td>y</td></tr></table>
</main>
</body></html>`

func TestExtractPricesEmails(t *testing.T) {
	entities := New().Extract([]byte(sample))

	var prices, emails, phones int
	var foundTitle bool
	var foundTable bool
	for _, e := range entities {
		switch e.Type {
		case "price":
			prices++
		case "email":
			emails++
		case "phone":
			phones++
		case "title":
			if e.Value == "Acme Product 3000" {
				foundTitle = true
			}
		case "table":
			foundTable = true
		}
	}
	if prices < 2 {
		t.Fatalf("expected >=2 prices, got %d", prices)
	}
	if emails != 1 {
		t.Fatalf("expected 1 email, got %d", emails)
	}
	if !foundTitle {
		t.Fatal("h1 not extracted as title")
	}
	if !foundTable {
		t.Fatal("table not extracted")
	}
	if phones < 1 {
		t.Fatalf("expected a phone, got %d", phones)
	}
}

func TestExtractorIgnoresCookieBanner(t *testing.T) {
	html := `<div id="cookie-banner">We use cookies. Accept?</div><main><h1>Real</h1></main>`
	entities := New().Extract([]byte(html))
	for _, e := range entities {
		if e.Value == "We use cookies. Accept?" {
			t.Fatal("cookie banner leaked into extraction")
		}
	}
}