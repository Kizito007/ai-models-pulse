package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"ai-models-pulse/internal/aggregator"
	"ai-models-pulse/internal/models"
)

type APIHandler struct {
	agg        *aggregator.Aggregator
	liveModels *models.LiveModelManager
}

func NewAPIHandler(agg *aggregator.Aggregator, liveModels *models.LiveModelManager) *APIHandler {
	return &APIHandler{
		agg:        agg,
		liveModels: liveModels,
	}
}

// FeedResponse payload
type FeedResponse struct {
	Items  []models.FeedItem `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// GetFeed handles /api/feed
func (h *APIHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	tag := r.URL.Query().Get("tag")
	source := r.URL.Query().Get("source")
	timeFilter := r.URL.Query().Get("time")
	sortBy := r.URL.Query().Get("sort")
	papersOnly := r.URL.Query().Get("papers") == "true" || r.URL.Query().Get("papers") == "1"

	limit := 40
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	offset := 0
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	items, total := h.agg.GetFilteredItems(q, category, tag, source, timeFilter, sortBy, papersOnly, limit, offset)

	writeJSON(w, http.StatusOK, FeedResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetModels handles /api/models with live sorting, global search & drop filtering
func (h *APIHandler) GetModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	typeFilter := r.URL.Query().Get("type")
	modalityFilter := r.URL.Query().Get("modality")
	providerFilter := r.URL.Query().Get("provider")
	q := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")
	dropsOnly := r.URL.Query().Get("drops") == "true"

	if dropsOnly {
		drops := h.liveModels.GetRecentDrops()
		writeJSON(w, http.StatusOK, drops)
		return
	}

	var results []models.AIModel
	if q != "" {
		// Global live search across Hugging Face & OpenRouter for ANY model in the world
		results = h.liveModels.GlobalSearch(q, typeFilter, modalityFilter, providerFilter, sortBy)
	} else {
		results = h.liveModels.GetFilteredModels("", typeFilter, modalityFilter, providerFilter, sortBy)
	}

	writeJSON(w, http.StatusOK, results)
}

// StreamLiveDrops handles /api/models/live-stream (Server-Sent Events)
func (h *APIHandler) StreamLiveDrops(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.liveModels.SubscribeDrops()
	defer h.liveModels.UnsubscribeDrops(ch)

	// Send initial ping
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"listening\"}\n\n")
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case model, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(model)
			if err == nil {
				fmt.Fprintf(w, "event: model_drop\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

// GetLatestDrops handles /api/models/latest-drops
func (h *APIHandler) GetLatestDrops(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	drops := h.liveModels.GetRecentDrops()
	writeJSON(w, http.StatusOK, drops)
}

// GetStats handles /api/stats
func (h *APIHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.agg.GetStats(h.liveModels.Count())
	writeJSON(w, http.StatusOK, stats)
}

// GetSources handles /api/sources
func (h *APIHandler) GetSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sources := h.agg.GetSources()
	writeJSON(w, http.StatusOK, sources)
}

// RefreshFeed handles POST /api/refresh (refreshes feeds & live models simultaneously)
func (h *APIHandler) RefreshFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	feedCount := h.agg.Refresh()
	modelCount := h.liveModels.SyncFromInternet()
	stats := h.agg.GetStats(modelCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"totalArticles": feedCount,
		"totalModels":   modelCount,
		"stats":         stats,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
