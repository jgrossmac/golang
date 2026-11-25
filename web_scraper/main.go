package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	WebsiteURL        string
	SearchText        string
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	EmailFrom         string
	EmailTo           string
	DiscordWebhookURL string
	CheckInterval     time.Duration
	RequireInStock    bool
	MaxAge            time.Duration // 0 = no max-age filter
}

func (c Config) EmailEnabled() bool {
	return c.SMTPHost != "" && c.EmailFrom != "" && c.EmailTo != ""
}

func (c Config) DiscordEnabled() bool {
	return c.DiscordWebhookURL != ""
}

type shopifyVariant struct {
	Available bool `json:"available"`
}

type shopifyProduct struct {
	Title       string           `json:"title"`
	Handle      string           `json:"handle"`
	Tags        []string         `json:"tags"`
	Variants    []shopifyVariant `json:"variants"`
	PublishedAt time.Time        `json:"published_at"`
}

func (p shopifyProduct) InStock() bool {
	for _, v := range p.Variants {
		if v.Available {
			return true
		}
	}
	return false
}

type shopifyResponse struct {
	Products []shopifyProduct `json:"products"`
}

func main() {
	_ = godotenv.Load()

	config := loadConfig()

	fmt.Printf("Starting web scraper...\n")
	fmt.Printf("Collection: %s\n", config.WebsiteURL)
	fmt.Printf("Search text: %s\n", config.SearchText)
	fmt.Printf("Check interval: %v\n", config.CheckInterval)
	fmt.Printf("Require in stock: %v\n", config.RequireInStock)
	if config.MaxAge > 0 {
		fmt.Printf("Max age: %v\n", config.MaxAge)
	} else {
		fmt.Printf("Max age: (disabled)\n")
	}
	fmt.Printf("Notifications: email=%v discord=%v\n", config.EmailEnabled(), config.DiscordEnabled())
	fmt.Println()

	checkCollection(config)

	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		checkCollection(config)
	}
}

func loadConfig() Config {
	websiteURL := getEnv("WEBSITE_URL", "")
	searchText := getEnv("SEARCH_TEXT", "")
	smtpHost := getEnv("SMTP_HOST", "")
	smtpPortStr := getEnv("SMTP_PORT", "587")
	smtpUsername := getEnv("SMTP_USERNAME", "")
	smtpPassword := getEnv("SMTP_PASSWORD", "")
	emailFrom := getEnv("EMAIL_FROM", "")
	emailTo := getEnv("EMAIL_TO", "")
	discordWebhookURL := getEnv("DISCORD_WEBHOOK_URL", "")
	intervalStr := getEnv("CHECK_INTERVAL", "5m")
	requireInStockStr := getEnv("REQUIRE_IN_STOCK", "false")
	maxAgeStr := getEnv("MAX_AGE", "24h")

	if websiteURL == "" {
		log.Fatal("WEBSITE_URL environment variable is required")
	}
	if searchText == "" {
		log.Fatal("SEARCH_TEXT environment variable is required")
	}

	// Email is configured iff all three of SMTP_HOST, EMAIL_FROM, EMAIL_TO are set;
	// partial configs are a mistake worth flagging.
	emailFields := []struct{ name, value string }{
		{"SMTP_HOST", smtpHost},
		{"EMAIL_FROM", emailFrom},
		{"EMAIL_TO", emailTo},
	}
	var emailSet, emailMissing []string
	for _, f := range emailFields {
		if f.value != "" {
			emailSet = append(emailSet, f.name)
		} else {
			emailMissing = append(emailMissing, f.name)
		}
	}
	if len(emailSet) > 0 && len(emailMissing) > 0 {
		log.Fatalf("Partial email config: %v set but %v missing. Set all three or none.", emailSet, emailMissing)
	}

	emailEnabled := len(emailMissing) == 0
	discordEnabled := discordWebhookURL != ""
	if !emailEnabled && !discordEnabled {
		log.Fatal("No notification channel configured. Set DISCORD_WEBHOOK_URL or all of SMTP_HOST/EMAIL_FROM/EMAIL_TO.")
	}

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		log.Fatalf("Invalid SMTP_PORT: %v", err)
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid CHECK_INTERVAL format: %v. Use format like '5m', '1h', etc.", err)
	}

	requireInStock, err := strconv.ParseBool(requireInStockStr)
	if err != nil {
		log.Fatalf("Invalid REQUIRE_IN_STOCK value %q: %v. Use 'true' or 'false'.", requireInStockStr, err)
	}

	var maxAge time.Duration
	if maxAgeStr != "" {
		maxAge, err = time.ParseDuration(maxAgeStr)
		if err != nil {
			log.Fatalf("Invalid MAX_AGE format: %v. Use format like '24h', '48h', '7d' (note: Go duration does not support 'd', use '168h' for 7 days).", err)
		}
		if maxAge <= 0 {
			log.Fatalf("MAX_AGE must be a positive duration; got %v", maxAge)
		}
	}

	return Config{
		WebsiteURL:        websiteURL,
		SearchText:        searchText,
		SMTPHost:          smtpHost,
		SMTPPort:          smtpPort,
		SMTPUsername:      smtpUsername,
		SMTPPassword:      smtpPassword,
		EmailFrom:         emailFrom,
		EmailTo:           emailTo,
		DiscordWebhookURL: discordWebhookURL,
		CheckInterval:     interval,
		RequireInStock:    requireInStock,
		MaxAge:            maxAge,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func checkCollection(config Config) {
	fmt.Printf("[%s] Checking collection...\n", time.Now().Format("2006-01-02 15:04:05"))

	base, err := url.Parse(config.WebsiteURL)
	if err != nil {
		log.Printf("Error parsing WEBSITE_URL: %v", err)
		return
	}

	products, err := fetchAllProducts(base)
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		return
	}
	fmt.Printf("Fetched %d products.\n", len(products))

	terms := parseSearchTerms(config.SearchText)
	if len(terms) == 0 {
		log.Printf("SEARCH_TEXT contained no usable terms after splitting on commas")
		return
	}
	matches := matchProducts(products, terms)
	if config.RequireInStock {
		before := len(matches)
		matches = filterInStock(matches)
		if dropped := before - len(matches); dropped > 0 {
			fmt.Printf("Filtered out %d out-of-stock match(es).\n", dropped)
		}
	}
	if config.MaxAge > 0 {
		before := len(matches)
		matches = filterByMaxAge(matches, time.Now(), config.MaxAge)
		if dropped := before - len(matches); dropped > 0 {
			fmt.Printf("Filtered out %d match(es) older than %v.\n", dropped, config.MaxAge)
		}
	}
	if len(matches) == 0 {
		fmt.Printf("No match found.\n")
		return
	}

	fmt.Printf("Match found! %d product(s).\n", len(matches))
	links := make([]string, 0, len(matches))
	for _, p := range matches {
		links = append(links, productURL(base, p.Handle)+"  ("+p.Title+")")
	}
	notify(config, config.SearchText, links)
}

func notify(config Config, matchText string, links []string) {
	if config.EmailEnabled() {
		sendEmailNotification(config, matchText, links)
	}
	if config.DiscordEnabled() {
		if err := sendDiscordNotification(config.DiscordWebhookURL, config.WebsiteURL, matchText, links); err != nil {
			log.Printf("Error sending Discord notification: %v", err)
		} else {
			fmt.Printf("Discord notification sent successfully!\n")
		}
	}
}

func parseSearchTerms(s string) []string {
	var out []string
	for _, raw := range strings.Split(s, ",") {
		if t := strings.TrimSpace(raw); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func filterInStock(products []shopifyProduct) []shopifyProduct {
	out := make([]shopifyProduct, 0, len(products))
	for _, p := range products {
		if p.InStock() {
			out = append(out, p)
		}
	}
	return out
}

// filterByMaxAge keeps only products whose PublishedAt is within maxAge of now.
// Products with a zero PublishedAt (missing/unparseable in the JSON) are dropped
// when the filter is active — we can't claim they're recent if we don't know.
func filterByMaxAge(products []shopifyProduct, now time.Time, maxAge time.Duration) []shopifyProduct {
	cutoff := now.Add(-maxAge)
	out := make([]shopifyProduct, 0, len(products))
	for _, p := range products {
		if p.PublishedAt.IsZero() {
			continue
		}
		if p.PublishedAt.After(cutoff) {
			out = append(out, p)
		}
	}
	return out
}

func matchProducts(products []shopifyProduct, terms []string) []shopifyProduct {
	lowered := make([]string, len(terms))
	for i, t := range terms {
		lowered[i] = strings.ToLower(t)
	}
	var matches []shopifyProduct
	for _, p := range products {
		title := strings.ToLower(p.Title)
		for _, needle := range lowered {
			if strings.Contains(title, needle) {
				matches = append(matches, p)
				break
			}
		}
	}
	return matches
}

// http.DefaultClient has no timeout, so a stalled connection would hang a
// check until the process is restarted.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// refusalDetail summarises a non-200 so a 403 says why. Stores explain
// themselves in the body and headers, and both were being discarded.
func refusalDetail(resp *http.Response) string {
	var parts []string
	for _, h := range []string{"Retry-After", "Server", "CF-Ray", "CF-Mitigated", "X-Request-Id"} {
		if v := resp.Header.Get(h); v != "" {
			parts = append(parts, h+"="+v)
		}
	}
	excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	if s := strings.Join(strings.Fields(string(excerpt)), " "); s != "" {
		parts = append(parts, "body="+strconv.Quote(s))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
}

func fetchAllProducts(collectionURL *url.URL) ([]shopifyProduct, error) {
	const pageLimit = 250
	var all []shopifyProduct
	for page := 1; ; page++ {
		jsonURL := *collectionURL
		jsonURL.Path = strings.TrimRight(jsonURL.Path, "/") + "/products.json"
		q := jsonURL.Query()
		q.Set("limit", strconv.Itoa(pageLimit))
		q.Set("page", strconv.Itoa(page))
		jsonURL.RawQuery = q.Encode()

		req, err := http.NewRequest("GET", jsonURL.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; web_scraper/1.0)")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body := resp.Body
		if resp.StatusCode != http.StatusOK {
			detail := refusalDetail(resp)
			body.Close()
			return nil, fmt.Errorf("status %d for %s%s", resp.StatusCode, jsonURL.String(), detail)
		}

		var parsed shopifyResponse
		err = json.NewDecoder(body).Decode(&parsed)
		body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode page %d: %w", page, err)
		}

		all = append(all, parsed.Products...)
		if len(parsed.Products) < pageLimit {
			break
		}
	}
	return all, nil
}

func sendDiscordNotification(webhookURL, websiteURL, matchText string, links []string) error {
	var body strings.Builder
	fmt.Fprintf(&body, "**Match Found: %s**\n", matchText)
	fmt.Fprintf(&body, "Site: %s\n", websiteURL)
	fmt.Fprintf(&body, "Time: %s", time.Now().Format("2006-01-02 15:04:05"))
	if len(links) > 0 {
		body.WriteString("\n\n**Links:**")
		for i, link := range links {
			fmt.Fprintf(&body, "\n%d. %s", i+1, link)
		}
	}

	payload, err := json.Marshal(map[string]string{"content": body.String()})
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()

	// Discord returns 204 No Content on success; some webhook variants return 200.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func productURL(base *url.URL, handle string) string {
	u := *base
	u.Path = "/products/" + handle
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func sendEmailNotification(config Config, matchText string, links []string) {
	var linksText strings.Builder
	if len(links) > 0 {
		linksText.WriteString("\n\nLinks:\n")
		for i, link := range links {
			linksText.WriteString(fmt.Sprintf("%d. %s\n", i+1, link))
		}
	}

	subject := fmt.Sprintf("Match Found: %s", matchText)
	body := fmt.Sprintf("Match Found!\n\nWebsite: %s\nSearch text: %s\nTime: %s%s",
		config.WebsiteURL, matchText, time.Now().Format("2006-01-02 15:04:05"), linksText.String())

	message := fmt.Sprintf("From: %s\r\n", config.EmailFrom)
	message += fmt.Sprintf("To: %s\r\n", config.EmailTo)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-Version: 1.0\r\n"
	message += "Content-Type: text/plain; charset=UTF-8\r\n"
	message += "\r\n"
	message += body

	var auth smtp.Auth
	if config.SMTPUsername != "" && config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	err := smtp.SendMail(addr, auth, config.EmailFrom, []string{config.EmailTo}, []byte(message))
	if err != nil {
		log.Printf("Error sending email: %v", err)
		return
	}

	fmt.Printf("Email notification sent successfully!\n")
}
