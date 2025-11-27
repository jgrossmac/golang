# Web Scraper with Email Notifications

A Go application that periodically scrapes a website for specific text and sends an email notification when a match is found.

## Features

- Scrapes website content for specified text
- Sends email notifications when matches are found
- Extracts and includes links to matching items
- Configurable check interval
- Environment variable configuration
- Automatic `.env` file support

## Prerequisites

- Go 1.18 or later
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
   WEBSITE_URL=https://example.com
   SEARCH_TEXT=your search text
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   SMTP_USERNAME=your-email@gmail.com
   SMTP_PASSWORD=your-app-password
   EMAIL_FROM=your-email@gmail.com
   EMAIL_TO=recipient@example.com
   CHECK_INTERVAL=5m
   ```
   
   The application will automatically load variables from `.env` if it exists.

   **Option B: Using environment variables:**
   ```bash
   export WEBSITE_URL="https://example.com"
   export SEARCH_TEXT="your search text"
   export SMTP_HOST="smtp.gmail.com"
   export SMTP_PORT="587"
   export SMTP_USERNAME="your-email@gmail.com"
   export SMTP_PASSWORD="your-app-password"
   export EMAIL_FROM="your-email@gmail.com"
   export EMAIL_TO="recipient@example.com"
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
- `SMTP_HOST` (required): SMTP server hostname (e.g., `smtp.gmail.com`)
- `SMTP_PORT` (required): SMTP server port (e.g., `587` for TLS, `465` for SSL)
- `SMTP_USERNAME` (optional): SMTP username (required for authenticated SMTP)
- `SMTP_PASSWORD` (optional): SMTP password or app password (required for authenticated SMTP)
- `EMAIL_FROM` (required): Email address to send from
- `EMAIL_TO` (required): Email address to send to
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
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USERNAME="your-email@gmail.com"
export SMTP_PASSWORD="your-app-password"
export EMAIL_FROM="your-email@gmail.com"
export EMAIL_TO="recipient@example.com"
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
     -e SMTP_HOST="smtp.gmail.com" \
     -e SMTP_PORT="587" \
     -e SMTP_USERNAME="your-email@gmail.com" \
     -e SMTP_PASSWORD="your-app-password" \
     -e EMAIL_FROM="your-email@gmail.com" \
     -e EMAIL_TO="recipient@example.com" \
     -e CHECK_INTERVAL="5m" \
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

- The scraper performs case-insensitive text matching
- The application runs continuously, checking at the specified interval
- All text content from the website's body is searched
- When a match is found, the email includes links to the matching items
- For Gmail, you must use an [App Password](https://support.google.com/accounts/answer/185833) instead of your regular password
- SMTP authentication is optional - if `SMTP_USERNAME` and `SMTP_PASSWORD` are not provided, the email will be sent without authentication (may not work with most providers)

