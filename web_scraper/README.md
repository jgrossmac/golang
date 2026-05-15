# Shopify Collection Watcher with Email Notifications

A Go application that periodically polls a Shopify collection's `/products.json`
endpoint for products matching one or more search terms and sends an email
notification when a match is found.

## How it works

Given a `WEBSITE_URL` that points at a Shopify collection page (e.g.
`https://store.example.com/collections/new-arrivals`), the app fetches
`<WEBSITE_URL>/products.json` (paginated, 250 at a time) and case-insensitively
matches each product's `title` against the configured search term(s). This is
much more reliable than HTML scraping — it ignores nav, footer, and
recommendation widgets, which were a common source of false positives in the
prior HTML-scraping implementation.

## Features

- Polls Shopify collection JSON for matching product titles
- Comma-separated list of search terms (OR-matched against each title)
- Optional `REQUIRE_IN_STOCK` filter using `variants[].available`
- Sends email notifications with direct `/products/<handle>` links
- Configurable check interval
- Environment variable / `.env` file configuration

## Prerequisites

- Go 1.18 or later
- The target site must be a Shopify storefront and expose
  `/collections/<slug>/products.json` (this is the default for Shopify;
  it does not work on non-Shopify sites)
- SMTP email server access (Gmail, Outlook, or custom SMTP server)

## Setup

1. **Configure SMTP Email Settings:**
   - For Gmail: Use `smtp.gmail.com` on port `587` with an [App Password](https://support.google.com/accounts/answer/185833)
   - For Outlook: Use `smtp-mail.outlook.com` on port `587`
   - For custom SMTP: Use your server's SMTP host and port

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Configure the application:**

   **Option A: Using a `.env` file (recommended):**
   
   Create a `.env` file in the project directory:
   ```bash
   WEBSITE_URL=https://store.example.com/collections/new-arrivals
   SEARCH_TEXT=patria, garrison brothers
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   SMTP_USERNAME=your-email@gmail.com
   SMTP_PASSWORD=your-app-password
   EMAIL_FROM=your-email@gmail.com
   EMAIL_TO=recipient@example.com
   CHECK_INTERVAL=5m
   REQUIRE_IN_STOCK=false
   ```
   
   The application will automatically load variables from `.env` if it exists.

   **Option B: Using environment variables:**
   ```bash
   export WEBSITE_URL="https://store.example.com/collections/new-arrivals"
   export SEARCH_TEXT="patria, garrison brothers"
   export SMTP_HOST="smtp.gmail.com"
   export SMTP_PORT="587"
   export SMTP_USERNAME="your-email@gmail.com"
   export SMTP_PASSWORD="your-app-password"
   export EMAIL_FROM="your-email@gmail.com"
   export EMAIL_TO="recipient@example.com"
   export CHECK_INTERVAL="5m"     # Optional, default is 5 minutes
   export REQUIRE_IN_STOCK="false" # Optional, default is false
   ```

4. **Run the application:**
   ```bash
   go run main.go
   ```

## Configuration

### Environment Variables

- `WEBSITE_URL` (required): The URL of a Shopify **collection** page
  (e.g. `https://store.example.com/collections/new-arrivals`). The app
  appends `/products.json` to this URL to fetch the collection's products.
- `SEARCH_TEXT` (required): One or more search terms, comma-separated.
  A product matches if any term is a case-insensitive substring of its title.
  Examples: `patria`  or  `patria, garrison brothers, veuve clicquot`.
- `SMTP_HOST` (required): SMTP server hostname (e.g., `smtp.gmail.com`)
- `SMTP_PORT` (required): SMTP server port (e.g., `587` for TLS, `465` for SSL)
- `SMTP_USERNAME` (optional): SMTP username (required for authenticated SMTP)
- `SMTP_PASSWORD` (optional): SMTP password or app password (required for authenticated SMTP)
- `EMAIL_FROM` (required): Email address to send from
- `EMAIL_TO` (required): Email address to send to
- `CHECK_INTERVAL` (optional): How often to check the website (e.g., "5m", "1h", "30s"). Default: "5m"
- `REQUIRE_IN_STOCK` (optional): If `true`, only notify on products that have
  at least one available variant (`variants[].available == true` in the JSON).
  Default: `false`.

### Check Interval Format

The `CHECK_INTERVAL` uses Go's duration format:
- `30s` = 30 seconds
- `5m` = 5 minutes
- `1h` = 1 hour
- `2h30m` = 2 hours 30 minutes

## Example Usage

```bash
export WEBSITE_URL="https://www.missionliquor.com/collections/new-item-alert"
export SEARCH_TEXT="patria, garrison brothers"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"
export EMAIL_FROM="your-email@gmail.com"
export EMAIL_TO="recipient@example.com"
export CHECK_INTERVAL="1m"
export REQUIRE_IN_STOCK="true"

go run main.go
```

### Watching multiple stores

To monitor more than one Shopify collection at a time, run multiple instances —
one per `.env` file. For example, with Docker:

```bash
docker run -d --env-file .env.mission --name scraper-mission web-scraper:latest
docker run -d --env-file .env.cypress --name scraper-cypress web-scraper:latest
docker run -d --env-file .env.hiproof --name scraper-hiproof web-scraper:latest
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
     -e WEBSITE_URL="https://store.example.com/collections/new-arrivals" \
     -e SEARCH_TEXT="patria, garrison brothers" \
     -e SMTP_HOST="smtp.gmail.com" \
     -e SMTP_PORT="587" \
     -e SMTP_USERNAME="your-email@gmail.com" \
     -e SMTP_PASSWORD="your-app-password" \
     -e EMAIL_FROM="your-email@gmail.com" \
     -e EMAIL_TO="recipient@example.com" \
     -e CHECK_INTERVAL="5m" \
     -e REQUIRE_IN_STOCK="false" \
     --name web-scraper \
     web-scraper:latest
   ```

   Or use a `.env` file:
   ```bash
   docker run -d --env-file .env --name web-scraper web-scraper:latest
   ```

## Email Provider Examples

### Gmail
```bash
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-16-char-app-password  # Generate at https://myaccount.google.com/apppasswords
EMAIL_FROM=your-email@gmail.com
EMAIL_TO=recipient@example.com
```

### Outlook/Hotmail
```bash
SMTP_HOST=smtp-mail.outlook.com
SMTP_PORT=587
SMTP_USERNAME=your-email@outlook.com
SMTP_PASSWORD=your-password
EMAIL_FROM=your-email@outlook.com
EMAIL_TO=recipient@example.com
```

### Custom SMTP Server
```bash
SMTP_HOST=mail.example.com
SMTP_PORT=587
SMTP_USERNAME=your-username
SMTP_PASSWORD=your-password
EMAIL_FROM=noreply@example.com
EMAIL_TO=recipient@example.com
```

## Notes

- Matching is case-insensitive and operates only on each product's `title`
  from the Shopify collection JSON — not on the rendered HTML — so nav,
  footer, and recommendation widgets cannot trigger false positives.
- `SEARCH_TEXT` supports a comma-separated list of terms; a product matches
  if any term is a substring of its title. Whitespace around each term is
  trimmed and empty entries are ignored.
- `REQUIRE_IN_STOCK=true` filters out products whose every variant has
  `available: false`. Default is `false` (notify on any matching product).
- The application runs continuously, checking at the specified interval.
  Matching products will trigger an email on every check while they remain
  in the collection — there is no de-duplication across runs.
- When a match is found, the email includes a direct
  `/products/<handle>` link for each matching product.
- The target site must be a Shopify storefront. The app will fail to parse
  the JSON response from non-Shopify sites.
- For Gmail, you must use an [App Password](https://support.google.com/accounts/answer/185833) instead of your regular password.
- SMTP authentication is optional — if `SMTP_USERNAME` and `SMTP_PASSWORD` are not provided, the email will be sent without authentication (may not work with most providers).

