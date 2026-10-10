package models

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type hfModelRaw struct {
	ID          string   `json:"id"`
	Likes       int      `json:"likes"`
	Downloads   int      `json:"downloads"`
	Tags        []string `json:"tags"`
	PipelineTag string   `json:"pipeline_tag"`
	LibraryName string   `json:"library_name"`
	CreatedAt   string   `json:"createdAt"`
}

type openRouterModelRaw struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Created       int64  `json:"created"`
	Description   string `json:"description"`
	ContextLength int    `json:"context_length"`
	Architecture  struct {
		Modality        string   `json:"modality"`
		Tokenizer       string   `json:"tokenizer"`
		InputModalities []string `json:"input_modalities"`
	} `json:"architecture"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

type openRouterResponse struct {
	Data []openRouterModelRaw `json:"data"`
}

type LiveModelManager struct {
	models        []AIModel
	recentDrops   []AIModel
	seenDropIDs   map[string]bool
	subscribers   map[chan AIModel]bool
	subMu         sync.Mutex
	mu            sync.RWMutex
	httpClient    *http.Client
	lastUpdated   time.Time
}

func NewLiveModelManager() *LiveModelManager {
	return &LiveModelManager{
		models:      make([]AIModel, 0),
		recentDrops: make([]AIModel, 0),
		seenDropIDs: make(map[string]bool),
		subscribers: make(map[chan AIModel]bool),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

var paramRegex = regexp.MustCompile(`(?i)([\d\.]+[BM])`)

// StartLiveDropWatcher polls for real-time model releases every 20 seconds
func (m *LiveModelManager) StartLiveDropWatcher() {
	ticker := time.NewTicker(20 * time.Second)
	go func() {
		for range ticker.C {
			m.pollNewestDrops()
		}
	}()
}

func (m *LiveModelManager) pollNewestDrops() {
	drops, err := m.fetchHFModels("https://huggingface.co/api/models?sort=createdAt&direction=-1&limit=15")
	if err != nil {
		return
	}

	m.mu.Lock()
	newFound := make([]AIModel, 0)
	for _, raw := range drops {
		if raw.ID == "" {
			continue
		}
		if !m.seenDropIDs[raw.ID] {
			m.seenDropIDs[raw.ID] = true
			model := transformHFModel(raw)
			newFound = append(newFound, model)
		}
	}

	if len(newFound) > 0 {
		// Prepend to recent drops
		m.recentDrops = append(newFound, m.recentDrops...)
		if len(m.recentDrops) > 50 {
			m.recentDrops = m.recentDrops[:50]
		}
		// Also add to global catalog
		m.models = append(newFound, m.models...)
	}
	m.mu.Unlock()

	// Broadcast to active SSE subscribers
	if len(newFound) > 0 {
		m.subMu.Lock()
		for ch := range m.subscribers {
			for _, item := range newFound {
				select {
				case ch <- item:
				default:
				}
			}
		}
		m.subMu.Unlock()
	}
}

// SubscribeDrops registers a channel for live SSE streaming
func (m *LiveModelManager) SubscribeDrops() chan AIModel {
	ch := make(chan AIModel, 20)
	m.subMu.Lock()
	m.subscribers[ch] = true
	m.subMu.Unlock()
	return ch
}

// UnsubscribeDrops cleans up a subscriber
func (m *LiveModelManager) UnsubscribeDrops(ch chan AIModel) {
	m.subMu.Lock()
	delete(m.subscribers, ch)
	close(ch)
	m.subMu.Unlock()
}

// SyncFromInternet fetches models from OpenRouter, Hugging Face (likes, downloads, latest), and curated
func (m *LiveModelManager) SyncFromInternet() int {
	log.Println("[LiveModelManager] Syncing live models from internet (Hugging Face + OpenRouter + Curated)...")

	modelMap := make(map[string]AIModel)

	// 1. Fetch live OpenRouter models (covers full Claude, GPT, Gemini, Grok, Mistral, Llama catalog)
	orModels, err := m.fetchOpenRouterModels()
	if err != nil {
		log.Printf("[LiveModelManager] Warning: failed to fetch OpenRouter models: %v\n", err)
	} else {
		for _, model := range orModels {
			modelMap[model.ID] = model
		}
		log.Printf("[LiveModelManager] Ingested %d live models from OpenRouter.\n", len(orModels))
	}

	// 2. Fetch top liked models from Hugging Face
	likedModels, err := m.fetchHFModels("https://huggingface.co/api/models?sort=likes&direction=-1&limit=100")
	if err != nil {
		log.Printf("[LiveModelManager] Warning: failed to fetch HF liked models: %v\n", err)
	}

	// 3. Fetch top downloaded models from Hugging Face
	downloadedModels, err := m.fetchHFModels("https://huggingface.co/api/models?sort=downloads&direction=-1&limit=100")
	if err != nil {
		log.Printf("[LiveModelManager] Warning: failed to fetch HF downloaded models: %v\n", err)
	}

	// 4. Fetch real-time latest created drops from Hugging Face
	createdModels, err := m.fetchHFModels("https://huggingface.co/api/models?sort=createdAt&direction=-1&limit=40")
	if err != nil {
		log.Printf("[LiveModelManager] Warning: failed to fetch HF created models: %v\n", err)
	}

	var dropsList []AIModel
	for _, raw := range append(append(likedModels, downloadedModels...), createdModels...) {
		if raw.ID == "" {
			continue
		}
		hfModel := transformHFModel(raw)
		if existing, exists := modelMap[raw.ID]; exists {
			existing.Likes = hfModel.Likes
			existing.Downloads = hfModel.Downloads
			existing.DownloadsStr = hfModel.DownloadsStr
			existing.IsLiveHF = true
			if len(hfModel.Links) > 0 {
				for k, v := range hfModel.Links {
					existing.Links[k] = v
				}
			}
			modelMap[raw.ID] = existing
		} else {
			modelMap[raw.ID] = hfModel
		}
	}

	for _, raw := range createdModels {
		if raw.ID != "" {
			dropsList = append(dropsList, transformHFModel(raw))
		}
	}

	// 5. Merge with curated registry benchmarks
	frontier := GetRegistryModels()
	for _, f := range frontier {
		matchedID := ""
		for id, m := range modelMap {
			if id == f.ID || strings.EqualFold(m.Name, f.Name) || strings.Contains(strings.ToLower(id), strings.ToLower(f.Name)) {
				matchedID = id
				break
			}
		}

		if matchedID != "" {
			existing := modelMap[matchedID]
			if len(f.Benchmarks) > 0 {
				existing.Benchmarks = f.Benchmarks
			}
			if len(f.KeyFeatures) > 0 {
				existing.KeyFeatures = f.KeyFeatures
			}
			if f.ContextLen != "" && existing.ContextLen == "" {
				existing.ContextLen = f.ContextLen
			}
			if f.Description != "" {
				existing.Description = f.Description
			}
			if f.Parameters != "Unknown" && f.Parameters != "" {
				existing.Parameters = f.Parameters
			}
			if f.BadgeColor != "" {
				existing.BadgeColor = f.BadgeColor
			}
			modelMap[matchedID] = existing
		} else {
			modelMap[f.ID] = f
		}
	}

	// Convert map to slice
	all := make([]AIModel, 0, len(modelMap))
	for _, model := range modelMap {
		all = append(all, model)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Likes != all[j].Likes {
			return all[i].Likes > all[j].Likes
		}
		return all[i].ContextNum > all[j].ContextNum
	})

	m.mu.Lock()
	m.models = all
	m.recentDrops = dropsList
	for _, d := range dropsList {
		m.seenDropIDs[d.ID] = true
	}
	m.lastUpdated = time.Now()
	total := len(m.models)
	m.mu.Unlock()

	log.Printf("[LiveModelManager] Successfully synced %d total live AI models from internet.\n", total)
	return total
}

func (m *LiveModelManager) fetchOpenRouterModels() ([]AIModel, error) {
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AI-Models-Pulse/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter api status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(body, &orResp); err != nil {
		return nil, err
	}

	var list []AIModel
	for _, raw := range orResp.Data {
		if strings.Contains(raw.ID, ":batch") || strings.HasPrefix(raw.ID, "~") {
			continue
		}

		parts := strings.Split(raw.ID, "/")
		provider := "AI Lab"
		cleanName := raw.Name
		if len(parts) == 2 {
			provider = formatProvider(parts[0])
		}
		cleanName = strings.TrimPrefix(cleanName, provider+": ")
		cleanName = strings.TrimPrefix(cleanName, "Anthropic: ")
		cleanName = strings.TrimPrefix(cleanName, "OpenAI: ")
		cleanName = strings.TrimPrefix(cleanName, "Google: ")
		cleanName = strings.TrimPrefix(cleanName, "Meta: ")

		contextK := raw.ContextLength / 1000
		contextStr := fmt.Sprintf("%dk tokens", contextK)
		if raw.ContextLength >= 1_000_000 {
			contextStr = fmt.Sprintf("%.1fM tokens", float64(raw.ContextLength)/1_000_000.0)
			contextStr = strings.ReplaceAll(contextStr, ".0M", "M")
		}

		modality := "Text & Code"
		if strings.Contains(raw.Architecture.Modality, "image") {
			modality = "Multimodal (Vision & Text)"
		}

		createdTime := time.Unix(raw.Created, 0)
		relDate := createdTime.Format("Jan 2006")

		mType := "Proprietary API"
		if strings.Contains(raw.ID, "meta-llama") || strings.Contains(raw.ID, "deepseek") || strings.Contains(raw.ID, "mistralai") || strings.Contains(raw.ID, "qwen") {
			mType = "Open Weights"
		}

		badgeColor := getBadgeColor(provider)

		desc := raw.Description
		if desc == "" {
			desc = fmt.Sprintf("Frontier AI model by %s with %s context window.", provider, contextStr)
		}

		list = append(list, AIModel{
			ID:          raw.ID,
			Name:        cleanName,
			Author:      provider,
			Provider:    provider,
			ReleaseDate: relDate,
			CreatedAt:   createdTime,
			Type:        mType,
			Modality:    modality,
			ContextLen:  contextStr,
			ContextNum:  contextK,
			License:     mType,
			Description: desc,
			Links: map[string]string{
				"openrouter": fmt.Sprintf("https://openrouter.ai/models/%s", raw.ID),
			},
			BadgeColor: badgeColor,
		})
	}
	return list, nil
}

func (m *LiveModelManager) fetchHFModels(targetUrl string) ([]hfModelRaw, error) {
	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AI-Models-Pulse/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huggingface api error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawList []hfModelRaw
	if err := json.Unmarshal(body, &rawList); err != nil {
		return nil, err
	}
	return rawList, nil
}

// GlobalSearch queries Hugging Face's global database of 1M+ models in real time and merges with OpenRouter
func (m *LiveModelManager) GlobalSearch(q, modelType, modality, provider, sortBy string) []AIModel {
	qClean := strings.TrimSpace(q)
	if qClean == "" {
		return m.GetFilteredModels("", modelType, modality, provider, sortBy)
	}

	// 1. Search in local in-memory catalog (covers OpenRouter + frontier models)
	localMatches := m.GetFilteredModels(qClean, modelType, modality, provider, sortBy)

	// 2. Perform live search on Hugging Face API for the entire world of models
	hfSearchUrl := fmt.Sprintf("https://huggingface.co/api/models?search=%s&limit=60", url.QueryEscape(qClean))
	hfRaws, err := m.fetchHFModels(hfSearchUrl)
	if err != nil {
		return localMatches
	}

	seen := make(map[string]bool)
	for _, l := range localMatches {
		seen[l.ID] = true
	}

	var results []AIModel
	results = append(results, localMatches...)

	for _, raw := range hfRaws {
		if raw.ID == "" || seen[raw.ID] {
			continue
		}
		model := transformHFModel(raw)
		seen[raw.ID] = true
		results = append(results, model)
	}

	// Sort
	switch strings.ToLower(sortBy) {
	case "downloads":
		sort.Slice(results, func(i, j int) bool { return results[i].Downloads > results[j].Downloads })
	case "latest":
		sort.Slice(results, func(i, j int) bool { return results[i].CreatedAt.After(results[j].CreatedAt) })
	case "name":
		sort.Slice(results, func(i, j int) bool { return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name) })
	case "likes":
		fallthrough
	default:
		sort.Slice(results, func(i, j int) bool { return results[i].Likes > results[j].Likes })
	}

	return results
}

func (m *LiveModelManager) GetRecentDrops() []AIModel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.recentDrops
}

func transformHFModel(raw hfModelRaw) AIModel {
	parts := strings.Split(raw.ID, "/")
	provider := "Community"
	name := raw.ID
	if len(parts) == 2 {
		provider = formatProvider(parts[0])
		name = parts[1]
	}

	license := "Open License"
	arxivID := ""
	for _, tag := range raw.Tags {
		if strings.HasPrefix(tag, "license:") {
			license = strings.TrimPrefix(tag, "license:")
		}
		if strings.HasPrefix(tag, "arxiv:") {
			arxivID = strings.TrimPrefix(tag, "arxiv:")
		}
	}

	params := "Unknown"
	if matches := paramRegex.FindStringSubmatch(name); len(matches) > 1 {
		params = strings.ToUpper(matches[1])
	}

	modality := "Text Generation"
	if raw.PipelineTag != "" {
		modality = formatPipelineTag(raw.PipelineTag)
	}

	var createdAt time.Time
	if raw.CreatedAt != "" {
		createdAt, _ = time.Parse(time.RFC3339, raw.CreatedAt)
	}
	relDate := ""
	if !createdAt.IsZero() {
		relDate = createdAt.Format("Jan 2006")
	}

	links := map[string]string{
		"huggingface": fmt.Sprintf("https://huggingface.co/%s", raw.ID),
	}
	if arxivID != "" {
		links["paper"] = fmt.Sprintf("https://arxiv.org/abs/%s", arxivID)
	}

	return AIModel{
		ID:           raw.ID,
		Name:         name,
		Author:       provider,
		Provider:     provider,
		ReleaseDate:  relDate,
		CreatedAt:    createdAt,
		Type:         "Open Weights",
		Modality:     modality,
		PipelineTag:  raw.PipelineTag,
		Parameters:   params,
		ContextLen:   "128k tokens",
		License:      license,
		Description:  fmt.Sprintf("Live open-weights AI model by %s hosted on Hugging Face. Active community usage across %s monthly downloads and %d likes.", provider, formatNumber(raw.Downloads), raw.Likes),
		Likes:        raw.Likes,
		Downloads:    raw.Downloads,
		DownloadsStr: formatNumber(raw.Downloads),
		ArxivID:      arxivID,
		Tags:         raw.Tags,
		Links:        links,
		BadgeColor:   getBadgeColor(provider),
		IsLiveHF:     true,
	}
}

func formatProvider(p string) string {
	switch strings.ToLower(p) {
	case "anthropic":
		return "Anthropic"
	case "openai":
		return "OpenAI"
	case "google", "google-deepmind", "deepmind":
		return "Google DeepMind"
	case "meta-llama", "meta":
		return "Meta AI"
	case "deepseek-ai", "deepseek":
		return "DeepSeek"
	case "qwen":
		return "Alibaba Cloud (Qwen)"
	case "mistralai", "mistral":
		return "Mistral AI"
	case "black-forest-labs":
		return "Black Forest Labs"
	case "x-ai", "xai":
		return "xAI"
	case "cohere":
		return "Cohere"
	case "microsoft":
		return "Microsoft"
	default:
		return strings.Title(p)
	}
}

func getBadgeColor(p string) string {
	switch strings.ToLower(p) {
	case "anthropic":
		return "amber"
	case "openai":
		return "teal"
	case "google", "google deepmind":
		return "blue"
	case "meta ai", "meta":
		return "purple"
	case "deepseek":
		return "emerald"
	case "alibaba cloud (qwen)", "qwen":
		return "rose"
	case "mistral ai", "mistral":
		return "orange"
	case "xai":
		return "red"
	default:
		return "cyan"
	}
}

func formatPipelineTag(tag string) string {
	switch tag {
	case "text-generation":
		return "Text Generation (LLM)"
	case "image-text-to-text":
		return "Multimodal (Vision & Text)"
	case "text-to-image":
		return "Image Generation"
	case "automatic-speech-recognition":
		return "Speech to Text"
	case "feature-extraction":
		return "Embeddings & Vectors"
	default:
		return strings.ReplaceAll(strings.Title(strings.ReplaceAll(tag, "-", " ")), "  ", " ")
	}
}

func formatNumber(num int) string {
	if num >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(num)/1_000_000.0)
	} else if num >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(num)/1_000.0)
	}
	return fmt.Sprintf("%d", num)
}

// GetFilteredModels returns models with sorting and filtering
func (m *LiveModelManager) GetFilteredModels(q, modelType, modality, provider, sortBy string) []AIModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	qLower := strings.ToLower(strings.TrimSpace(q))
	typeLower := strings.ToLower(strings.TrimSpace(modelType))
	modalityLower := strings.ToLower(strings.TrimSpace(modality))
	providerLower := strings.ToLower(strings.TrimSpace(provider))

	var list []AIModel
	for _, model := range m.models {
		if typeLower != "" && typeLower != "all" {
			if !strings.Contains(strings.ToLower(model.Type), typeLower) {
				continue
			}
		}

		if modalityLower != "" && modalityLower != "all" {
			if !strings.Contains(strings.ToLower(model.Modality), modalityLower) &&
				!strings.Contains(strings.ToLower(model.PipelineTag), modalityLower) {
				continue
			}
		}

		if providerLower != "" && providerLower != "all" {
			provMatch := strings.Contains(strings.ToLower(model.Provider), providerLower) ||
				strings.Contains(strings.ToLower(model.ID), providerLower) ||
				strings.Contains(strings.ToLower(model.Name), providerLower)
			if !provMatch {
				continue
			}
		}

		if qLower != "" {
			combined := strings.ToLower(model.Name + " " + model.Provider + " " + model.Description + " " + model.Parameters + " " + model.License + " " + model.ID)
			if !strings.Contains(combined, qLower) {
				continue
			}
		}

		list = append(list, model)
	}

	// Sort
	switch strings.ToLower(sortBy) {
	case "downloads":
		sort.Slice(list, func(i, j int) bool { return list[i].Downloads > list[j].Downloads })
	case "latest":
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	case "name":
		sort.Slice(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	case "likes":
		fallthrough
	default:
		sort.Slice(list, func(i, j int) bool {
			if list[i].Likes != list[j].Likes {
				return list[i].Likes > list[j].Likes
			}
			return list[i].ContextNum > list[j].ContextNum
		})
	}

	return list
}

func (m *LiveModelManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.models)
}
