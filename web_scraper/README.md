# Web Scraper with Discord Notifications

A Go application that periodically scrapes a website for specific text and sends a Discord notification when a match is found.

## Features

- Scrapes website content for specified text
- Sends Discord webhook notifications when matches are found
- Configurable check interval
- Environment variable configuration
- Automatic `.env` file support

## Prerequisites

- Go 1.19 or later
- A Discord webhook URL

## Setup

1. **Get a Discord Webhook URL:**
   - Go to your Discord server settings
   - Navigate to Integrations → Webhooks
   - Create a new webhook and copy the webhook URL

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Configure the application:**

   **Option A: Using a `.env` file (recommended):**
   
   Create a `.env` file in the project directory (you can copy `.env.example` as a template):
   ```bash
   cp .env.example .env
   # Then edit .env with your values
   ```
   
   Or create it manually:
   ```bash
   WEBSITE_URL=https://example.com
   SEARCH_TEXT=your search text
   DISCORD_WEBHOOK=https://discord.com/api/webhooks/...
   CHECK_INTERVAL=5m
   ```
   
   The application will automatically load variables from `.env` if it exists.

   **Option B: Using environment variables:**
   ```bash
   export WEBSITE_URL="https://example.com"
   export SEARCH_TEXT="your search text"
   export DISCORD_WEBHOOK="https://discord.com/api/webhooks/..."
   export CHECK_INTERVAL="5m"  # Optional, default is 5 minutes
   ```

4. **Run the application:**
   ```bash
   go run main.go
   ```

## Configuration

### Environment Variables

- `WEBSITE_URL` (required): The URL of the website to scrape
- `SEARCH_TEXT` (required): The text to search for on the website
- `DISCORD_WEBHOOK` (required): Your Discord webhook URL
- `CHECK_INTERVAL` (optional): How often to check the website (e.g., "5m", "1h", "30s"). Default: "5m"

### Check Interval Format

The `CHECK_INTERVAL` uses Go's duration format:
- `30s` = 30 seconds
- `5m` = 5 minutes
- `1h` = 1 hour
- `2h30m` = 2 hours 30 minutes

## Example Usage

```bash
export WEBSITE_URL="https://example.com/products"
export SEARCH_TEXT="in stock"
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/123456789/abcdefgh"
export CHECK_INTERVAL="1m"

go run main.go
```

## Building

To build a binary:

```bash
go build -o web_scraper
./web_scraper
```

## Docker Deployment

1. **Build the Docker image:**
   ```bash
   docker build -t web-scraper:latest .
   ```

2. **Run the container:**
   ```bash
   docker run -d \
     -e WEBSITE_URL="https://example.com" \
     -e SEARCH_TEXT="your search text" \
     -e DISCORD_WEBHOOK="https://discord.com/api/webhooks/..." \
     -e CHECK_INTERVAL="5m" \
     --name web-scraper \
     web-scraper:latest
   ```

   Or use a `.env` file:
   ```bash
   docker run -d --env-file .env --name web-scraper web-scraper:latest
   ```

## Kubernetes Deployment

1. **Update the configuration files:**
   - Edit `k8s-deployment.yaml` and update the ConfigMap with your `WEBSITE_URL`, `SEARCH_TEXT`, and `CHECK_INTERVAL`
   - Update the Secret with your `DISCORD_WEBHOOK` (or use `kubectl create secret`)

2. **Build and push the Docker image:**
   ```bash
   docker build -t your-registry/web-scraper:latest .
   docker push your-registry/web-scraper:latest
   ```

3. **Update the image in k8s-deployment.yaml:**
   - Change `image: web-scraper:latest` to your image path

4. **Create the Secret (if not using the one in the file):**
   ```bash
   kubectl create secret generic web-scraper-secrets \
     --from-literal=DISCORD_WEBHOOK="https://discord.com/api/webhooks/..."
   ```

5. **Deploy to Kubernetes:**
   ```bash
   kubectl apply -f k8s-deployment.yaml
   ```

6. **Check the deployment:**
   ```bash
   kubectl get pods -l app=web-scraper
   kubectl logs -l app=web-scraper
   ```

## Notes

- The scraper performs case-insensitive text matching
- The application runs continuously, checking at the specified interval
- All text content from the website's body is searched

