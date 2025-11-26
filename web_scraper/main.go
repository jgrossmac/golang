package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
)

type Config struct {
	WebsiteURL     string
	SearchText     string
	DiscordWebhook string
	CheckInterval  time.Duration
}

type DiscordWebhook struct {
	Content string `json:"content"`
}

func main() {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	config := loadConfig()

	fmt.Printf("Starting web scraper...\n")
	fmt.Printf("Website: %s\n", config.WebsiteURL)
	fmt.Printf("Search text: %s\n", config.SearchText)
	fmt.Printf("Check interval: %v\n", config.CheckInterval)
	fmt.Println()

	// Run initial check
	checkWebsite(config)

	// Set up ticker for periodic checks
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkWebsite(config)
	}
}

func loadConfig() Config {
	websiteURL := getEnv("WEBSITE_URL", "")
	searchText := getEnv("SEARCH_TEXT", "")
	discordWebhook := getEnv("DISCORD_WEBHOOK", "")
	intervalStr := getEnv("CHECK_INTERVAL", "5m")

	if websiteURL == "" {
		log.Fatal("WEBSITE_URL environment variable is required")
	}
	if searchText == "" {
		log.Fatal("SEARCH_TEXT environment variable is required")
	}
	if discordWebhook == "" {
		log.Fatal("DISCORD_WEBHOOK environment variable is required")
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("Invalid CHECK_INTERVAL format: %v. Use format like '5m', '1h', etc.", err)
	}

	return Config{
		WebsiteURL:     websiteURL,
		SearchText:     searchText,
		DiscordWebhook: discordWebhook,
		CheckInterval:  interval,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func checkWebsite(config Config) {
	fmt.Printf("[%s] Checking website...\n", time.Now().Format("2006-01-02 15:04:05"))

	// Fetch the webpage
	resp, err := http.Get(config.WebsiteURL)
	if err != nil {
		log.Printf("Error fetching website: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: received status code %d", resp.StatusCode)
		return
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Error parsing HTML: %v", err)
		return
	}

	// Extract all text content for quick check
	var textContent strings.Builder
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		textContent.WriteString(s.Text())
	})

	// Check if search text is found
	pageText := textContent.String()
	searchTextLower := strings.ToLower(config.SearchText)
	if !strings.Contains(strings.ToLower(pageText), searchTextLower) {
		fmt.Printf("No match found.\n")
		return
	}

	fmt.Printf("Match found! Extracting links...\n")

	// Find elements containing the search text and extract their links
	links := findLinksForText(doc, config.WebsiteURL, searchTextLower)

	if len(links) > 0 {
		sendDiscordNotification(config, config.SearchText, links)
	} else {
		// If no specific links found, just use the base URL
		sendDiscordNotification(config, config.SearchText, []string{config.WebsiteURL})
	}
}

func findLinksForText(doc *goquery.Document, baseURL string, searchTextLower string) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("Error parsing base URL: %v", err)
		return nil
	}

	// Check if we're already on a product page and the text matches
	// If so, return the current page URL as the link
	if strings.Contains(baseURL, "/products/") {
		// Check if the search text appears on this product page
		bodyText := strings.ToLower(doc.Find("body").Text())
		if strings.Contains(bodyText, searchTextLower) {
			return []string{baseURL}
		}
	}

	linkMap := make(map[string]bool)
	var productLinks []string
	var otherLinks []string

	// Strategy 1: Find all <a> tags that directly contain the search text
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, searchTextLower) {
			if href, exists := s.Attr("href"); exists {
				resolved := resolveURL(base, href)
				if resolved != "" && !linkMap[resolved] {
					linkMap[resolved] = true
					// Prioritize product links
					if strings.Contains(resolved, "/products/") {
						productLinks = append(productLinks, resolved)
					} else {
						otherLinks = append(otherLinks, resolved)
					}
				}
			}
		}
	})

	// Strategy 2: Find elements containing the text, then look for the closest link
	// This handles cases where the text is in headings, product titles, etc.
	// Look in common product-related selectors first
	productSelectors := []string{"h1", "h2", "h3", "[class*='product']", "[class*='item']", "[id*='product']"}
	for _, selector := range productSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.ToLower(s.Text())
			if strings.Contains(text, searchTextLower) {
				link := findClosestLink(s, base)
				if link != "" && !linkMap[link] {
					linkMap[link] = true
					if strings.Contains(link, "/products/") {
						productLinks = append(productLinks, link)
					} else {
						otherLinks = append(otherLinks, link)
					}
				}
			}
		})
	}

	// Strategy 3: General search for any element containing the text
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		// Check if this element's direct text contains the search text
		directText := strings.ToLower(s.Clone().Children().Remove().End().Text())
		if strings.Contains(directText, searchTextLower) {
			// Prioritize parent links (text inside a link)
			link := findClosestLink(s, base)
			if link != "" && !linkMap[link] {
				linkMap[link] = true
				// Prioritize product links
				if strings.Contains(link, "/products/") {
					productLinks = append(productLinks, link)
				} else {
					otherLinks = append(otherLinks, link)
				}
			}
		}
	})

	// Return product links first, then other links
	if len(productLinks) > 0 {
		return productLinks
	}
	return otherLinks
}

func findClosestLink(s *goquery.Selection, baseURL *url.URL) string {
	// Check if the element itself is a link
	if s.Is("a") {
		if href, exists := s.Attr("href"); exists {
			return resolveURL(baseURL, href)
		}
	}

	// Check parent links first (most common case: text is inside a link)
	var foundLink string
	s.Parents().Each(func(i int, parent *goquery.Selection) {
		if foundLink != "" {
			return
		}
		if parent.Is("a") {
			if href, exists := parent.Attr("href"); exists {
				foundLink = resolveURL(baseURL, href)
			}
		}
	})
	if foundLink != "" {
		return foundLink
	}

	// Check for link children
	s.Find("a").First().Each(func(i int, link *goquery.Selection) {
		if href, exists := link.Attr("href"); exists {
			foundLink = resolveURL(baseURL, href)
		}
	})
	if foundLink != "" {
		return foundLink
	}

	// Check parent containers for links (common in product listings)
	s.Parents().Each(func(i int, parent *goquery.Selection) {
		if foundLink != "" {
			return
		}
		// Look for links in the parent container
		parent.Find("a").First().Each(func(i int, link *goquery.Selection) {
			if href, exists := link.Attr("href"); exists {
				resolved := resolveURL(baseURL, href)
				// Prioritize product links
				if strings.Contains(resolved, "/products/") {
					foundLink = resolved
				} else if foundLink == "" {
					foundLink = resolved
				}
			}
		})
	})

	return foundLink
}

func resolveURL(baseURL *url.URL, href string) string {
	if href == "" {
		return ""
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// Resolve relative URLs
	resolved := baseURL.ResolveReference(parsed)
	return resolved.String()
}

func sendDiscordNotification(config Config, matchText string, links []string) {
	var linksText strings.Builder
	if len(links) > 0 {
		linksText.WriteString("\n\n**Links:**\n")
		for i, link := range links {
			linksText.WriteString(fmt.Sprintf("%d. %s\n", i+1, link))
		}
	}

	message := fmt.Sprintf("🔔 **Match Found!**\n\nWebsite: %s\nSearch text: %s\nTime: %s%s",
		config.WebsiteURL, matchText, time.Now().Format("2006-01-02 15:04:05"), linksText.String())

	webhook := DiscordWebhook{
		Content: message,
	}

	jsonData, err := json.Marshal(webhook)
	if err != nil {
		log.Printf("Error marshaling webhook data: %v", err)
		return
	}

	resp, err := http.Post(config.DiscordWebhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending Discord notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("Discord notification sent successfully!\n")
	} else {
		log.Printf("Error: Discord webhook returned status code %d", resp.StatusCode)
	}
}
