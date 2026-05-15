package main

import (
	"encoding/json"
	"fmt"
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
	WebsiteURL     string
	SearchText     string
	SMTPHost       string
	SMTPPort       int
	SMTPUsername   string
	SMTPPassword   string
	EmailFrom      string
	EmailTo        string
	CheckInterval  time.Duration
	RequireInStock bool
}

type shopifyVariant struct {
	Available bool `json:"available"`
}

type shopifyProduct struct {
	Title    string           `json:"title"`
	Handle   string           `json:"handle"`
	Tags     []string         `json:"tags"`
	Variants []shopifyVariant `json:"variants"`
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
	intervalStr := getEnv("CHECK_INTERVAL", "5m")
	requireInStockStr := getEnv("REQUIRE_IN_STOCK", "false")

	if websiteURL == "" {
		log.Fatal("WEBSITE_URL environment variable is required")
	}
	if searchText == "" {
		log.Fatal("SEARCH_TEXT environment variable is required")
	}
	if smtpHost == "" {
		log.Fatal("SMTP_HOST environment variable is required")
	}
	if emailFrom == "" {
		log.Fatal("EMAIL_FROM environment variable is required")
	}
	if emailTo == "" {
		log.Fatal("EMAIL_TO environment variable is required")
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

	return Config{
		WebsiteURL:     websiteURL,
		SearchText:     searchText,
		SMTPHost:       smtpHost,
		SMTPPort:       smtpPort,
		SMTPUsername:   smtpUsername,
		SMTPPassword:   smtpPassword,
		EmailFrom:      emailFrom,
		EmailTo:        emailTo,
		CheckInterval:  interval,
		RequireInStock: requireInStock,
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
	if len(matches) == 0 {
		fmt.Printf("No match found.\n")
		return
	}

	fmt.Printf("Match found! %d product(s).\n", len(matches))
	links := make([]string, 0, len(matches))
	for _, p := range matches {
		links = append(links, productURL(base, p.Handle)+"  ("+p.Title+")")
	}
	sendEmailNotification(config, config.SearchText, links)
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

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body := resp.Body
		if resp.StatusCode != http.StatusOK {
			body.Close()
			return nil, fmt.Errorf("status %d for %s", resp.StatusCode, jsonURL.String())
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
