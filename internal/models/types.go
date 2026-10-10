package models

import "time"

type FeedItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Link        string    `json:"link"`
	Source      string    `json:"source"`
	SourceIcon  string    `json:"sourceIcon"`
	Author      string    `json:"author,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	TimeAgo     string    `json:"timeAgo"`
	Categories  []string  `json:"categories"`
	ModelTags   []string  `json:"modelTags"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	Upvotes     int       `json:"upvotes,omitempty"`
	GithubRepo  string    `json:"githubRepo,omitempty"`
	IsPaper     bool      `json:"isPaper"`
}

type AIModel struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Author       string            `json:"author"`
	Provider     string            `json:"provider"`
	ReleaseDate  string            `json:"releaseDate"`
	CreatedAt    time.Time         `json:"createdAt"`
	LastModified time.Time         `json:"lastModified"`
	Type         string            `json:"type"`     // "Open Weights", "Proprietary"
	Modality     string            `json:"modality"` // "Text Generation", "Multimodal", "Image Generation", "Audio"
	PipelineTag  string            `json:"pipelineTag"`
	Parameters   string            `json:"parameters"`
	ContextLen   string            `json:"contextLen"`
	ContextNum   int               `json:"contextNum"`
	License      string            `json:"license"`
	Description  string            `json:"description"`
	Likes        int               `json:"likes"`
	Downloads    int               `json:"downloads"`
	DownloadsStr string            `json:"downloadsStr"`
	ArxivID      string            `json:"arxivId,omitempty"`
	Tags         []string          `json:"tags"`
	KeyFeatures  []string          `json:"keyFeatures"`
	Benchmarks   map[string]string `json:"benchmarks"`
	Links        map[string]string `json:"links"`
	BadgeColor   string            `json:"badgeColor"`
	IsLiveHF     bool              `json:"isLiveHF"`
}

type FeedSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Type     string `json:"type"` // "rss", "atom", "hf_api"
	Category string `json:"category"`
	Icon     string `json:"icon"`
	Enabled  bool   `json:"enabled"`
}

type Stats struct {
	TotalArticles  int            `json:"totalArticles"`
	TotalPapers    int            `json:"totalPapers"`
	TotalModels    int            `json:"totalModels"`
	ActiveFeeds    int            `json:"activeFeeds"`
	LastUpdated    time.Time      `json:"lastUpdated"`
	LastUpdatedStr string         `json:"lastUpdatedStr"`
	TrendingModels map[string]int `json:"trendingModels"`
}
