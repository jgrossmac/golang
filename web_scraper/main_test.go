package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestMatchProducts_FindsByTitleSubstring(t *testing.T) {
	products := []shopifyProduct{
		{Title: "Patria Avoyelles 2022 750ml", Handle: "patria-avoyelles-2022-750ml"},
		{Title: "1000 Stories Chardonnay 2022 750ml", Handle: "1000-stories"},
		{Title: "Garrison Brothers Guadalupe Straight Bourbon 750ml", Handle: "garrison"},
	}
	got := matchProducts(products, []string{"garrison"})
	if len(got) != 1 || got[0].Handle != "garrison" {
		t.Fatalf("expected 1 garrison match, got %+v", got)
	}
}

func TestMatchProducts_CaseInsensitive(t *testing.T) {
	products := []shopifyProduct{{Title: "Patria Avoyelles 2022 750ml", Handle: "p"}}
	if got := matchProducts(products, []string{"PATRIA"}); len(got) != 1 {
		t.Fatalf("case-insensitive match failed: %+v", got)
	}
}

// Regression test for the original false-positive bug: the old scraper matched against
// the entire HTML body, so search terms like "stock" or "notify me" hit nav/footer/
// recommendation widgets. With JSON-only matching against product titles, terms that
// do not appear in any product title must return zero matches.
func TestMatchProducts_NoFalsePositivesFromPageChrome(t *testing.T) {
	products := []shopifyProduct{
		{Title: "Patria Avoyelles 2022 750ml", Handle: "p"},
		{Title: "Veuve Clicquot Yellow Label 750ml", Handle: "v"},
	}
	for _, needle := range []string{"stock", "notify me", "add to cart", "sign in", "out of stock"} {
		if got := matchProducts(products, []string{needle}); len(got) != 0 {
			t.Errorf("expected no matches for page-chrome term %q; got %+v", needle, got)
		}
	}
}

func TestMatchProducts_NoMatch(t *testing.T) {
	products := []shopifyProduct{{Title: "Patria Avoyelles 2022 750ml", Handle: "p"}}
	if got := matchProducts(products, []string{"macallan"}); len(got) != 0 {
		t.Fatalf("expected no match, got %+v", got)
	}
}

func TestMatchProducts_MultipleTermsOr(t *testing.T) {
	products := []shopifyProduct{
		{Title: "Patria Avoyelles 2022 750ml", Handle: "patria"},
		{Title: "1000 Stories Chardonnay 2022 750ml", Handle: "stories"},
		{Title: "Garrison Brothers Guadalupe Straight Bourbon 750ml", Handle: "garrison"},
		{Title: "Veuve Clicquot Yellow Label 750ml", Handle: "veuve"},
	}
	got := matchProducts(products, []string{"patria", "garrison"})
	if len(got) != 2 {
		t.Fatalf("expected 2 matches across two terms, got %d: %+v", len(got), got)
	}
	handles := map[string]bool{got[0].Handle: true, got[1].Handle: true}
	if !handles["patria"] || !handles["garrison"] {
		t.Fatalf("expected patria+garrison handles, got %+v", got)
	}
}

// A product that matches multiple search terms must only appear once in the result.
func TestMatchProducts_DedupesWhenMultipleTermsHitSameProduct(t *testing.T) {
	products := []shopifyProduct{
		{Title: "Garrison Brothers Guadalupe Straight Bourbon 750ml", Handle: "garrison"},
	}
	got := matchProducts(products, []string{"garrison", "bourbon", "guadalupe"})
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped match, got %d: %+v", len(got), got)
	}
}

func TestInStock(t *testing.T) {
	cases := []struct {
		name string
		p    shopifyProduct
		want bool
	}{
		{"no variants", shopifyProduct{}, false},
		{"single available", shopifyProduct{Variants: []shopifyVariant{{Available: true}}}, true},
		{"single unavailable", shopifyProduct{Variants: []shopifyVariant{{Available: false}}}, false},
		{"any-available wins", shopifyProduct{Variants: []shopifyVariant{{Available: false}, {Available: true}}}, true},
		{"all unavailable", shopifyProduct{Variants: []shopifyVariant{{Available: false}, {Available: false}}}, false},
	}
	for _, c := range cases {
		if got := c.p.InStock(); got != c.want {
			t.Errorf("%s: InStock() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFilterInStock(t *testing.T) {
	in := []shopifyProduct{
		{Handle: "a", Variants: []shopifyVariant{{Available: true}}},
		{Handle: "b", Variants: []shopifyVariant{{Available: false}}},
		{Handle: "c", Variants: []shopifyVariant{{Available: false}, {Available: true}}},
		{Handle: "d"}, // no variants
	}
	got := filterInStock(in)
	if len(got) != 2 || got[0].Handle != "a" || got[1].Handle != "c" {
		t.Fatalf("filterInStock = %+v; want only a, c", got)
	}
}

// Verify that combining matchProducts + filterInStock (the REQUIRE_IN_STOCK=true path
// in checkCollection) drops out-of-stock matches while keeping in-stock ones.
func TestMatchThenFilterInStock(t *testing.T) {
	products := []shopifyProduct{
		{Title: "Patria Avoyelles 2022 750ml", Handle: "patria-instock", Variants: []shopifyVariant{{Available: true}}},
		{Title: "Patria Limited Reserve 750ml", Handle: "patria-oos", Variants: []shopifyVariant{{Available: false}}},
		{Title: "Garrison Brothers 750ml", Handle: "garrison", Variants: []shopifyVariant{{Available: true}}},
	}
	matched := matchProducts(products, []string{"patria"})
	if len(matched) != 2 {
		t.Fatalf("expected 2 patria matches before stock filter, got %d", len(matched))
	}
	filtered := filterInStock(matched)
	if len(filtered) != 1 || filtered[0].Handle != "patria-instock" {
		t.Fatalf("expected only patria-instock after filter, got %+v", filtered)
	}
}

func TestParseSearchTerms(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"patria", []string{"patria"}},
		{"patria,garrison", []string{"patria", "garrison"}},
		{"  patria  ,  garrison  ", []string{"patria", "garrison"}},
		{"patria,,garrison", []string{"patria", "garrison"}},
		{" , , ", nil},
		{"", nil},
		{"multi word, another term", []string{"multi word", "another term"}},
	}
	for _, c := range cases {
		got := parseSearchTerms(c.in)
		if len(got) != len(c.want) {
			t.Errorf("parseSearchTerms(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseSearchTerms(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestProductURL_DropsCollectionPathAndQuery(t *testing.T) {
	base, err := url.Parse("https://www.missionliquor.com/collections/new-item-alert?foo=1#bar")
	if err != nil {
		t.Fatal(err)
	}
	got := productURL(base, "garrison-brothers-750ml")
	want := "https://www.missionliquor.com/products/garrison-brothers-750ml"
	if got != want {
		t.Fatalf("productURL: got %q, want %q", got, want)
	}
}

// Build a fake Shopify-style collection endpoint that serves N products across
// paginated /products.json responses, the same way the real site does.
func newFakeShopify(t *testing.T, total int) (*httptest.Server, *url.URL) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/products.json") {
			http.NotFound(w, r)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if limit <= 0 || page <= 0 {
			http.Error(w, "missing limit/page", http.StatusBadRequest)
			return
		}
		start := (page - 1) * limit
		end := start + limit
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		out := make([]shopifyProduct, 0, end-start)
		for i := start; i < end; i++ {
			out = append(out, shopifyProduct{
				Title:  fmt.Sprintf("Product %d", i),
				Handle: fmt.Sprintf("p-%d", i),
			})
		}
		_ = json.NewEncoder(w).Encode(shopifyResponse{Products: out})
	}))
	t.Cleanup(srv.Close)
	base, err := url.Parse(srv.URL + "/collections/new-item-alert")
	if err != nil {
		t.Fatal(err)
	}
	return srv, base
}

func TestFetchAllProducts_Paginates(t *testing.T) {
	const total = 567 // forces 3 pages at limit=250
	_, base := newFakeShopify(t, total)

	got, err := fetchAllProducts(base)
	if err != nil {
		t.Fatalf("fetchAllProducts: %v", err)
	}
	if len(got) != total {
		t.Fatalf("expected %d products, got %d", total, len(got))
	}
	if got[0].Title != "Product 0" || got[total-1].Title != fmt.Sprintf("Product %d", total-1) {
		t.Fatalf("pagination assembled wrong: first=%q last=%q", got[0].Title, got[total-1].Title)
	}
}

func TestFetchAllProducts_SinglePage(t *testing.T) {
	_, base := newFakeShopify(t, 3)

	got, err := fetchAllProducts(base)
	if err != nil {
		t.Fatalf("fetchAllProducts: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 products, got %d", len(got))
	}
}

// End-to-end against a fake Shopify collection: a needle matching one product title
// returns exactly that product, with the right /products/<handle> URL.
func TestFetchAndMatch_EndToEnd(t *testing.T) {
	_, base := newFakeShopify(t, 500)

	products, err := fetchAllProducts(base)
	if err != nil {
		t.Fatalf("fetchAllProducts: %v", err)
	}
	// With 500 products (0..499), "Product 42" is a substring of "Product 42" and
	// "Product 420".."Product 429" — 11 matches total. Exercises both pagination
	// and substring matching end-to-end.
	matches := matchProducts(products, []string{"Product 42"})
	if len(matches) != 11 {
		t.Fatalf("expected 11 matches for 'Product 42', got %d: %+v", len(matches), matches)
	}
	gotURL := productURL(base, matches[0].Handle)
	if !strings.HasSuffix(gotURL, "/products/p-42") {
		t.Fatalf("unexpected product URL: %q", gotURL)
	}
}
