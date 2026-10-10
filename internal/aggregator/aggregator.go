package aggregator

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-models-pulse/internal/models"
)

type Aggregator struct {
	sources     []models.FeedSource
	items       []models.FeedItem
	mu          sync.RWMutex
	lastUpdated time.Time
	httpClient  *http.Client
}

func NewAggregator() *Aggregator {
	sources := []models.FeedSource{
		{
			ID:       "hf-papers",
			Name:     "HF Daily Papers",
			URL:      "https://huggingface.co/api/daily_papers",
			Type:     "hf_api",
			Category: "Research Papers",
			Icon:     "🤗",
			Enabled:  true,
		},
		{
			ID:       "hf-blog",
			Name:     "Hugging Face Blog",
			URL:      "https://huggingface.co/blog/feed.xml",
			Type:     "rss",
			Category: "Open Weights & Models",
			Icon:     "🤗",
			Enabled:  true,
		},
		{
			ID:       "deepmind",
			Name:     "Google DeepMind",
			URL:      "https://deepmind.google/blog/rss.xml",
			Type:     "rss",
			Category: "Frontier Research",
			Icon:     "✨",
			Enabled:  true,
		},
		{
			ID:       "openai",
			Name:     "OpenAI News",
			URL:      "https://openai.com/news/rss.xml",
			Type:     "rss",
			Category: "Frontier Research",
			Icon:     "🔮",
			Enabled:  true,
		},
		{
			ID:       "arxiv-cl",
			Name:     "arXiv cs.CL (Language & LLMs)",
			URL:      "https://export.arxiv.org/rss/cs.CL",
			Type:     "rss",
			Category: "Research Papers",
			Icon:     "📄",
			Enabled:  true,
		},
		{
			ID:       "arxiv-ai",
			Name:     "arXiv cs.AI (Artificial Intelligence)",
			URL:      "https://export.arxiv.org/rss/cs.AI",
			Type:     "rss",
			Category: "Research Papers",
			Icon:     "📄",
			Enabled:  true,
		},
		{
			ID:       "techcrunch-ai",
			Name:     "TechCrunch AI",
			URL:      "https://techcrunch.com/category/artificial-intelligence/feed/",
			Type:     "rss",
			Category: "Industry News",
			Icon:     "⚡",
			Enabled:  true,
		},
		{
			ID:       "mit-tech-review",
			Name:     "MIT Technology Review",
			URL:      "https://www.technologyreview.com/topic/artificial-intelligence/feed",
			Type:     "rss",
			Category: "Analysis & Policy",
			Icon:     "🏛️",
			Enabled:  true,
		},
		{
			ID:       "the-verge-ai",
			Name:     "The Verge AI",
			URL:      "https://www.theverge.com/rss/ai-artificial-intelligence/index.xml",
			Type:     "atom",
			Category: "Industry News",
			Icon:     "📰",
			Enabled:  true,
		},
	}

	return &Aggregator{
		sources: sources,
		items:   make([]models.FeedItem, 0),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

// StartAutoRefresh starts a periodic background worker
func (a *Aggregator) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	// First fetch immediately
	a.Refresh()

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				a.Refresh()
			}
		}
	}()
}

// Refresh fetches all sources concurrently
func (a *Aggregator) Refresh() int {
	log.Println("[Aggregator] Starting feed sync...")
	var wg sync.WaitGroup
	itemChan := make(chan []models.FeedItem, len(a.sources))

	for _, src := range a.sources {
		if !src.Enabled {
			continue
		}
		wg.Add(1)
		go func(s models.FeedSource) {
			defer wg.Done()
			items, err := a.fetchSource(s)
			if err != nil {
				log.Printf("[Aggregator] Warning: failed to fetch source %s: %v\n", s.Name, err)
				return
			}
			itemChan <- items
		}(src)
	}

	wg.Wait()
	close(itemChan)

	// Collect and deduplicate
	seenLinks := make(map[string]bool)
	seenTitles := make(map[string]bool)
	var combined []models.FeedItem

	for batch := range itemChan {
		for _, item := range batch {
			cleanTitle := strings.ToLower(strings.TrimSpace(item.Title))
			if cleanTitle == "" || seenTitles[cleanTitle] {
				continue
			}
			if item.Link != "" && seenLinks[item.Link] {
				continue
			}

			seenTitles[cleanTitle] = true
			if item.Link != "" {
				seenLinks[item.Link] = true
			}
			combined = append(combined, item)
		}
	}

	// Sort newest first
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].PublishedAt.After(combined[j].PublishedAt)
	})

	// Recompute relative time
	for i := range combined {
		combined[i].TimeAgo = FormatTimeAgo(combined[i].PublishedAt)
	}

	a.mu.Lock()
	a.items = combined
	a.lastUpdated = time.Now()
	total := len(a.items)
	a.mu.Unlock()

	log.Printf("[Aggregator] Feed sync complete. %d articles indexed.\n", total)
	return total
}

func (a *Aggregator) fetchSource(src models.FeedSource) ([]models.FeedItem, error) {
	req, err := http.NewRequest("GET", src.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AI-Models-Pulse/1.0 (compatible; aggregator)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/json, text/xml, */*")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, src.URL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return ParseFeed(body, src)
}

// GetFilteredItems returns items filtered by query, category, model tag, source, time range, and sorted
func (a *Aggregator) GetFilteredItems(search, category, modelTag, source, timeFilter, sortBy string, papersOnly bool, limit, offset int) ([]models.FeedItem, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	searchLower := strings.ToLower(strings.TrimSpace(search))
	catLower := strings.ToLower(strings.TrimSpace(category))
	tagLower := strings.ToLower(strings.TrimSpace(modelTag))
	sourceLower := strings.ToLower(strings.TrimSpace(source))

	now := time.Now()
	var timeThreshold time.Time
	switch strings.ToLower(timeFilter) {
	case "24h":
		timeThreshold = now.Add(-24 * time.Hour)
	case "7d":
		timeThreshold = now.Add(-7 * 24 * time.Hour)
	case "30d":
		timeThreshold = now.Add(-30 * 24 * time.Hour)
	}

	var matched []models.FeedItem

	for _, item := range a.items {
		if papersOnly && !item.IsPaper {
			continue
		}

		if !timeThreshold.IsZero() && item.PublishedAt.Before(timeThreshold) {
			continue
		}

		if searchLower != "" {
			fullContent := strings.ToLower(item.Title + " " + item.Summary + " " + item.Author + " " + item.Source)
			if !strings.Contains(fullContent, searchLower) {
				continue
			}
		}

		if catLower != "" && catLower != "all" {
			hasCat := false
			for _, c := range item.Categories {
				if strings.ToLower(c) == catLower {
					hasCat = true
					break
				}
			}
			if !hasCat {
				continue
			}
		}

		if tagLower != "" && tagLower != "all" {
			hasTag := false
			for _, t := range item.ModelTags {
				if strings.ToLower(t) == tagLower {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		if sourceLower != "" && sourceLower != "all" {
			if strings.ToLower(item.Source) != sourceLower {
				continue
			}
		}

		matched = append(matched, item)
	}

	// Sort matched items
	switch strings.ToLower(sortBy) {
	case "oldest":
		sort.Slice(matched, func(i, j int) bool { return matched[i].PublishedAt.Before(matched[j].PublishedAt) })
	case "upvotes":
		sort.Slice(matched, func(i, j int) bool { return matched[i].Upvotes > matched[j].Upvotes })
	case "title":
		sort.Slice(matched, func(i, j int) bool { return strings.ToLower(matched[i].Title) < strings.ToLower(matched[j].Title) })
	case "latest":
		fallthrough
	default:
		sort.Slice(matched, func(i, j int) bool { return matched[i].PublishedAt.After(matched[j].PublishedAt) })
	}

	totalMatches := len(matched)

	if offset >= totalMatches {
		return []models.FeedItem{}, totalMatches
	}

	end := offset + limit
	if limit <= 0 || end > totalMatches {
		end = totalMatches
	}

	return matched[offset:end], totalMatches
}

// GetSources returns configured feed sources
func (a *Aggregator) GetSources() []models.FeedSource {
	return a.sources
}

// GetStats compiles real-time feed statistics
func (a *Aggregator) GetStats(modelCount int) models.Stats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	paperCount := 0
	trending := make(map[string]int)

	for _, item := range a.items {
		if item.IsPaper {
			paperCount++
		}
		for _, tag := range item.ModelTags {
			trending[tag]++
		}
	}

	lastStr := "Never"
	if !a.lastUpdated.IsZero() {
		lastStr = FormatTimeAgo(a.lastUpdated)
	}

	return models.Stats{
		TotalArticles:  len(a.items),
		TotalPapers:    paperCount,
		TotalModels:    modelCount,
		ActiveFeeds:    len(a.sources),
		LastUpdated:    a.lastUpdated,
		LastUpdatedStr: lastStr,
		TrendingModels: trending,
	}
}
