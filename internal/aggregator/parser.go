package aggregator

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"ai-models-pulse/internal/models"
)

// RSS XML structures
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Author      string `xml:"author"`
	Creator     string `xml:"creator"` // dc:creator
	GUID        string `xml:"guid"`
}

// Atom XML structures
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	ID        string     `xml:"id"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Links     []atomLink `xml:"link"`
	Author    atomAuthor `xml:"author"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

// HF Paper JSON structure
type hfPaperEntry struct {
	Paper struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		Upvotes    int    `json:"upvotes"`
		GithubRepo string `json:"githubRepo"`
		Authors    []struct {
			Name string `json:"name"`
		} `json:"authors"`
		PublishedAt string `json:"publishedAt"`
	} `json:"paper"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	PublishedAt string `json:"publishedAt"`
	Thumbnail   string `json:"thumbnail"`
}

// ParseFeed detects format and parses raw bytes into FeedItems
func ParseFeed(data []byte, source models.FeedSource) ([]models.FeedItem, error) {
	if source.Type == "hf_api" {
		return parseHFPapers(data, source)
	}

	// Try RSS first
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err == nil && (len(rss.Channel.Items) > 0 || rss.Channel.Title != "" || rss.XMLName.Local == "rss") {
		return parseRSS(rss, source), nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err == nil && (len(atom.Entries) > 0 || atom.Title != "" || atom.XMLName.Local == "feed") {
		return parseAtom(atom, source), nil
	}

	return nil, fmt.Errorf("unrecognized XML feed structure for source %s", source.Name)
}

func parseRSS(rss rssFeed, source models.FeedSource) []models.FeedItem {
	var items []models.FeedItem
	for _, raw := range rss.Channel.Items {
		title := CleanText(raw.Title)
		summary := CleanText(raw.Description)
		if title == "" {
			continue
		}

		link := strings.TrimSpace(raw.Link)
		pubTime := parseFlexibleDate(raw.PubDate)

		author := raw.Creator
		if author == "" {
			author = raw.Author
		}

		id := raw.GUID
		if id == "" {
			id = hashString(link + title)
		}

		isPaper := strings.Contains(source.Name, "arXiv") || strings.Contains(title, "arXiv:")
		modelTags := ExtractModelTags(title, summary)
		cats := Categorize(title, summary, source.Name, isPaper)

		items = append(items, models.FeedItem{
			ID:          id,
			Title:       title,
			Summary:     truncate(summary, 280),
			Link:        link,
			Source:      source.Name,
			SourceIcon:  source.Icon,
			Author:      author,
			PublishedAt: pubTime,
			TimeAgo:     FormatTimeAgo(pubTime),
			Categories:  cats,
			ModelTags:   modelTags,
			IsPaper:     isPaper,
		})
	}
	return items
}

func parseAtom(atom atomFeed, source models.FeedSource) []models.FeedItem {
	var items []models.FeedItem
	for _, entry := range atom.Entries {
		title := CleanText(entry.Title)
		summary := CleanText(entry.Summary)
		if summary == "" {
			summary = CleanText(entry.Content)
		}
		if title == "" {
			continue
		}

		link := ""
		for _, l := range entry.Links {
			if l.Rel == "alternate" || l.Rel == "" {
				link = l.Href
				break
			}
		}
		if link == "" && len(entry.Links) > 0 {
			link = entry.Links[0].Href
		}

		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}
		pubTime := parseFlexibleDate(dateStr)

		id := entry.ID
		if id == "" {
			id = hashString(link + title)
		}

		isPaper := strings.Contains(source.Name, "arXiv") || strings.Contains(title, "arXiv:")
		modelTags := ExtractModelTags(title, summary)
		cats := Categorize(title, summary, source.Name, isPaper)

		items = append(items, models.FeedItem{
			ID:          id,
			Title:       title,
			Summary:     truncate(summary, 280),
			Link:        link,
			Source:      source.Name,
			SourceIcon:  source.Icon,
			Author:      entry.Author.Name,
			PublishedAt: pubTime,
			TimeAgo:     FormatTimeAgo(pubTime),
			Categories:  cats,
			ModelTags:   modelTags,
			IsPaper:     isPaper,
		})
	}
	return items
}

func parseHFPapers(data []byte, source models.FeedSource) ([]models.FeedItem, error) {
	var rawPapers []hfPaperEntry
	if err := json.Unmarshal(data, &rawPapers); err != nil {
		return nil, fmt.Errorf("hf paper json parse failed: %w", err)
	}

	var items []models.FeedItem
	for _, entry := range rawPapers {
		title := entry.Paper.Title
		if title == "" {
			title = entry.Title
		}
		title = CleanText(title)

		summary := entry.Paper.Summary
		if summary == "" {
			summary = entry.Summary
		}
		summary = CleanText(summary)

		if title == "" {
			continue
		}

		dateStr := entry.Paper.PublishedAt
		if dateStr == "" {
			dateStr = entry.PublishedAt
		}
		pubTime := parseFlexibleDate(dateStr)

		var authors []string
		for _, a := range entry.Paper.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
			if len(authors) >= 3 {
				break
			}
		}
		authorStr := strings.Join(authors, ", ")

		paperID := entry.Paper.ID
		paperLink := fmt.Sprintf("https://huggingface.co/papers/%s", paperID)

		modelTags := ExtractModelTags(title, summary)
		cats := Categorize(title, summary, "Hugging Face Daily Papers", true)

		items = append(items, models.FeedItem{
			ID:          hashString("hf-paper-" + paperID),
			Title:       title,
			Summary:     truncate(summary, 320),
			Link:        paperLink,
			Source:      "HF Daily Papers",
			SourceIcon:  "🤗",
			Author:      authorStr,
			PublishedAt: pubTime,
			TimeAgo:     FormatTimeAgo(pubTime),
			Categories:  cats,
			ModelTags:   modelTags,
			Thumbnail:   entry.Thumbnail,
			Upvotes:     entry.Paper.Upvotes,
			GithubRepo:  entry.Paper.GithubRepo,
			IsPaper:     true,
		})
	}
	return items, nil
}

func parseFlexibleDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}

	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 GMT",
		"2006-01-02",
	}

	trimmed := strings.TrimSpace(dateStr)
	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t
		}
	}
	return time.Now()
}

func truncate(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen-3]) + "..."
}

func hashString(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
