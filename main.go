package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Article represents an earnings news article
type Article struct {
	Title       string
	URL         string
	Description string
	Date        string
	ParsedDate  time.Time
}

// fetchHTML fetches HTML content from the given URL
func fetchHTML(url string) (*goquery.Document, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}

	return doc, nil
}

// parseDate attempts to parse various date formats from the article
func parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Clean up the date string
	dateStr = strings.TrimSpace(dateStr)

	// Common date patterns found in Japanese financial news
	patterns := []string{
		"01/02 15:04",      // MM/DD HH:MM
		"01/02",            // MM/DD
		"2006/01/02 15:04", // YYYY/MM/DD HH:MM
		"2006/01/02",       // YYYY/MM/DD
		"2006-01-02 15:04", // YYYY-MM-DD HH:MM
		"2006-01-02",       // YYYY-MM-DD
	}

	// Try to extract date using regex patterns
	regexPatterns := []string{
		`(\d{1,2})/(\d{1,2})\s+(\d{1,2}):(\d{2})`,         // MM/DD HH:MM
		`(\d{1,2})/(\d{1,2})`,                             // MM/DD
		`(\d{4})/(\d{1,2})/(\d{1,2})\s+(\d{1,2}):(\d{2})`, // YYYY/MM/DD HH:MM
		`(\d{4})/(\d{1,2})/(\d{1,2})`,                     // YYYY/MM/DD
	}

	currentYear := time.Now().Year()

	for i, pattern := range regexPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(dateStr)

		if len(matches) > 0 {
			switch i {
			case 0: // MM/DD HH:MM
				if len(matches) >= 5 {
					month, _ := strconv.Atoi(matches[1])
					day, _ := strconv.Atoi(matches[2])
					hour, _ := strconv.Atoi(matches[3])
					minute, _ := strconv.Atoi(matches[4])
					return time.Date(currentYear, time.Month(month), day, hour, minute, 0, 0, time.Local)
				}
			case 1: // MM/DD
				if len(matches) >= 3 {
					month, _ := strconv.Atoi(matches[1])
					day, _ := strconv.Atoi(matches[2])
					return time.Date(currentYear, time.Month(month), day, 0, 0, 0, 0, time.Local)
				}
			case 2: // YYYY/MM/DD HH:MM
				if len(matches) >= 6 {
					year, _ := strconv.Atoi(matches[1])
					month, _ := strconv.Atoi(matches[2])
					day, _ := strconv.Atoi(matches[3])
					hour, _ := strconv.Atoi(matches[4])
					minute, _ := strconv.Atoi(matches[5])
					return time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.Local)
				}
			case 3: // YYYY/MM/DD
				if len(matches) >= 4 {
					year, _ := strconv.Atoi(matches[1])
					month, _ := strconv.Atoi(matches[2])
					day, _ := strconv.Atoi(matches[3])
					return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
				}
			}
		}
	}

	// Try standard Go time parsing as fallback
	for _, pattern := range patterns {
		if parsedTime, err := time.Parse(pattern, dateStr); err == nil {
			// If year is missing, assume current year
			if parsedTime.Year() == 0 {
				parsedTime = time.Date(currentYear, parsedTime.Month(), parsedTime.Day(),
					parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second(),
					parsedTime.Nanosecond(), parsedTime.Location())
			}
			return parsedTime
		}
	}

	return time.Time{}
}

// extractDateFromURL attempts to extract date from URL patterns like /news/n202510010929
func extractDateFromURL(url string) time.Time {
	// Pattern for kabutan URLs: /news/n + YYYYMMDD + sequence
	re := regexp.MustCompile(`/news/n(\d{8})\d+`)
	matches := re.FindStringSubmatch(url)

	if len(matches) >= 2 {
		dateStr := matches[1]
		if len(dateStr) == 8 {
			year, _ := strconv.Atoi(dateStr[:4])
			month, _ := strconv.Atoi(dateStr[4:6])
			day, _ := strconv.Atoi(dateStr[6:8])
			return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		}
	}

	return time.Time{}
}

// parseEarningsNews extracts earnings news articles from the HTML document
func parseEarningsNews(doc *goquery.Document) ([]Article, error) {
	var articles []Article

	// Debug: Print page title and basic structure
	fmt.Println("ページタイトル:", strings.TrimSpace(doc.Find("title").Text()))
	fmt.Println("記事を検索中...")

	// Look for various possible article containers
	// Try different selectors that might contain news articles
	selectors := []string{
		"a[href*='/news/']", // Links to news articles
		"a[href*='news']",   // General news links
		".news-item",        // News item class
		".article-item",     // Article item class
		"tr td a",           // Table rows with links
		"div a",             // Div elements with links
		"li a",              // List items with links
	}

	articleMap := make(map[string]Article) // Use map to avoid duplicates

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			// Get link URL first
			link, exists := s.Attr("href")
			if !exists {
				return
			}

			// Ensure absolute URL
			if strings.HasPrefix(link, "/") {
				link = "https://us.kabutan.jp" + link
			}

			// Skip non-news URLs
			if !strings.Contains(link, "news") && !strings.Contains(link, "earnings") {
				return
			}

			// Get title from link text
			title := strings.TrimSpace(s.Text())
			if title == "" {
				return
			}

			// Skip if title is too short or seems like navigation
			if len(title) < 10 || strings.Contains(title, "ログイン") || strings.Contains(title, "登録") {
				return
			}

			// Get parent element for additional context
			parent := s.Parent()
			description := ""
			date := ""

			// Try to find date information
			parent.Find("time, .date, .published").Each(func(j int, dateEl *goquery.Selection) {
				dateText := strings.TrimSpace(dateEl.Text())
				if dateText != "" && date == "" {
					date = dateText
				}
			})

			// Use the link URL as key to avoid duplicates
			if _, exists := articleMap[link]; !exists {
				parsedDate := parseDate(date)

				// If no date found, try to extract from URL (e.g., /news/n202510010929)
				if parsedDate.IsZero() {
					parsedDate = extractDateFromURL(link)
				}

				articleMap[link] = Article{
					Title:       title,
					URL:         link,
					Description: description,
					Date:        date,
					ParsedDate:  parsedDate,
				}
			}
		})
	}

	// Convert map to slice
	for _, article := range articleMap {
		articles = append(articles, article)
	}

	fmt.Printf("発見された記事数: %d件\n", len(articles))

	return articles, nil
}

// filterGrowthArticles filters articles that contain growth-related keywords
func filterGrowthArticles(articles []Article) []Article {
	var filtered []Article

	// Keywords related to revenue and profit growth
	growthKeywords := []string{
		"増収増益",
		"増収営業増益",
		"好調",
		"上方修正",
		"増加",
	}

	for _, article := range articles {
		// Check if title contains any growth keywords
		titleLower := strings.ToLower(article.Title)
		descLower := strings.ToLower(article.Description)

		for _, keyword := range growthKeywords {
			if strings.Contains(article.Title, keyword) ||
				strings.Contains(article.Description, keyword) ||
				strings.Contains(titleLower, strings.ToLower(keyword)) ||
				strings.Contains(descLower, strings.ToLower(keyword)) {
				filtered = append(filtered, article)
				break // Avoid duplicates
			}
		}
	}

	return filtered
}

// filterByDate filters articles to only include those from the last 2 days
func filterByDate(articles []Article, daysBack int) []Article {
	var filtered []Article

	// Calculate the cutoff date (current date minus specified days)
	cutoffDate := time.Now().AddDate(0, 0, -daysBack)

	fmt.Printf("日付フィルター: %s以降の記事を検索\n", cutoffDate.Format("2006/01/02"))

	for _, article := range articles {
		// If we have a parsed date, use it for filtering
		if !article.ParsedDate.IsZero() {
			if article.ParsedDate.After(cutoffDate) || article.ParsedDate.Equal(cutoffDate) {
				filtered = append(filtered, article)
			}
		} else {
			// If no date could be parsed, include the article (to be safe)
			filtered = append(filtered, article)
		}
	}

	return filtered
}

// displayArticles prints the filtered articles
func displayArticles(articles []Article) {
	fmt.Printf("「増収増益」関連の決算ニュース記事: %d件\n", len(articles))
	fmt.Println(strings.Repeat("=", 80))

	if len(articles) == 0 {
		fmt.Println("該当する記事が見つかりませんでした。")
		return
	}

	for i, article := range articles {
		fmt.Printf("%d. %s\n", i+1, cleanTitle(article.Title))
		fmt.Printf("   URL: %s\n", article.URL)

		// Display date information
		if !article.ParsedDate.IsZero() {
			fmt.Printf("   日付: %s", article.ParsedDate.Format("2006/01/02 15:04"))
			if article.Date != "" {
				fmt.Printf(" (元の表記: %s)", article.Date)
			}
			fmt.Println()
		} else if article.Date != "" {
			fmt.Printf("   日付: %s\n", article.Date)
		}

		// Try to get more details from the article
		if articleDetails := getArticleDetails(article.URL); articleDetails != "" {
			fmt.Printf("   詳細: %s\n", articleDetails)
		}

		fmt.Println()
	}
}

// cleanTitle removes extra whitespace and formatting from title
func cleanTitle(title string) string {
	// Remove excessive whitespace
	cleaned := strings.ReplaceAll(title, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\t", " ")

	// Replace multiple spaces with single space
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}

	return strings.TrimSpace(cleaned)
}

// getArticleDetails fetches additional details from the article page
func getArticleDetails(url string) string {
	doc, err := fetchHTML(url)
	if err != nil {
		return ""
	}

	// Try to extract the main content or summary
	content := ""

	// Look for article content in various possible containers
	contentSelectors := []string{
		".article-content",
		".news-content",
		"main",
		".content",
		"article",
	}

	for _, selector := range contentSelectors {
		if element := doc.Find(selector).First(); element.Length() > 0 {
			text := strings.TrimSpace(element.Text())
			if len(text) > 50 { // Only use if substantial content
				content = text
				break
			}
		}
	}

	// Limit content length for display
	if len(content) > 300 {
		content = content[:300] + "..."
	}

	return content
}

func main() {
	fmt.Println("株探の決算ニュースから「増収増益」記事を検索中...")

	// Fetch HTML from the earnings news page
	doc, err := fetchHTML("https://us.kabutan.jp/earnings_news")
	if err != nil {
		log.Fatalf("HTMLの取得に失敗しました: %v", err)
	}

	// Parse earnings news articles
	articles, err := parseEarningsNews(doc)
	if err != nil {
		log.Fatalf("記事の解析に失敗しました: %v", err)
	}

	fmt.Printf("総記事数: %d件\n", len(articles))

	// Filter articles by date (last 2 days)
	recentArticles := filterByDate(articles, 2)
	fmt.Printf("過去2日間の記事数: %d件\n", len(recentArticles))

	// Filter articles containing growth keywords
	growthArticles := filterGrowthArticles(recentArticles)

	// Display results
	displayArticles(growthArticles)
}
