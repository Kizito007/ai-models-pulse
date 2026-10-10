# 🚀 AI Models Pulse - 100% Live Internet AI Observatory

A high-performance web platform built in **Go** that tracks, indexes, and categorizes the latest happenings in Artificial Intelligence with **100% live data directly from the internet**, emphasizing **AI Models** (DeepSeek, Gemini, Claude, GPT/OpenAI, Llama, Qwen, Mistral, Flux, and more).

---

## 🌐 100% Live Internet Data Architecture

Every piece of data across the platform is fetched live from public internet feeds and APIs:

1. **Live Model Radar & Directory**:
   - Pulled live from the **Hugging Face Models API** (`https://huggingface.co/api/models`) and official lab releases.
   - Live metrics:
     - **Live Community Stars / Likes** (e.g. ❤️ 17.4K, 14.3K)
     - **Live Monthly Downloads** (e.g. 📥 6.8M, 1.1M)
     - **Live Task & Pipeline Classification** (`Text Generation`, `Multimodal (Vision & Text)`, `Image Generation`, `Audio`)
     - **Live License extraction** (`apache-2.0`, `mit`, `llama3.1`, etc.)
     - **Direct ArXiv Paper integration** (`arXiv:2501.12948`)
     - **Direct Hub Links** to weights and code repos.
   - Sorters: Most Liked, Most Downloaded, Recently Released, Name (A-Z).
   - Filters: Open Weights, Proprietary, Text/LLMs, Vision/Multimodal, Image Generation.

2. **Live Model Comparison Tool**:
   - Compare any 3 models simultaneously using live internet metrics:
     - Community Stars, Monthly Downloads, Tasks, Parameters, Context Windows, Licenses, ArXiv papers, and Benchmarks.

3. **Search & Live Filtering Engine**:
   - **Sort By**:
     - ⏱️ Latest / Newest First
     - 🔥 Most Upvoted / Popular
     - ⏳ Oldest First
     - 🔤 Title (A - Z)
   - **Time Range**:
     - All Time, Past 24 Hours, Past 7 Days, Past 30 Days
   - **Live Source Filter**:
     - Hugging Face Daily Papers, Hugging Face Blog, Google DeepMind, OpenAI News, arXiv cs.CL, arXiv cs.AI, TechCrunch, MIT Technology Review, The Verge.
   - **Category Pills**:
     - LLMs & Reasoning, Open Weights, Multimodal & Vision, Research Papers, Agents & Code, Hardware & Compute.
   - **Model Tags**:
     - DeepSeek, Gemini, Claude, GPT/OpenAI, Llama, Qwen, Mistral, Flux/Diffusion.

---

## 🏃 Quick Start

### Run the server
```powershell
cd C:\Users\Thinker\.gemini\antigravity\scratch\ai-models-pulse
.\server.exe
# OR: go run main.go
```

Open your browser at:
👉 **[http://localhost:8080](http://localhost:8080)**

---

## 📡 Live REST API Reference

| Endpoint | Method | Description | Example Query |
| :--- | :--- | :--- | :--- |
| `/api/feed` | `GET` | Live feed items with sort & time filter | `/api/feed?sort=upvotes&time=24h&limit=20` |
| `/api/models` | `GET` | Live models with likes, downloads, and tags | `/api/models?sort=downloads&type=Open` |
| `/api/stats` | `GET` | Real-time counts & trending tags | `/api/stats` |
| `/api/sources` | `GET` | Active feed sources and connection status | `/api/sources` |
| `/api/refresh` | `POST` | Instant sync of all feeds and models | `/api/refresh` |

---

## 📁 Source Tree

```
ai-models-pulse/
├── main.go                     # Server entrypoint, goroutines, embedded static FS
├── go.mod                      # Module definition
├── internal/
│   ├── aggregator/
│   │   ├── aggregator.go       # Concurrent live feed fetcher, deduplicator, cache
│   │   ├── parser.go           # RSS 2.0, Atom 1.0, and HF JSON API parsers
│   │   └── tagger.go           # Auto-detection of models and categories
│   ├── handlers/
│   │   └── api.go              # REST API controllers
│   └── models/
│       ├── types.go            # Data types with live likes and downloads
│       ├── live_fetcher.go     # Live Hugging Face API model fetcher & enricher
│       └── registry.go         # Frontier model specifications & benchmarks
└── web/
    └── static/
        ├── index.html          # High-tech cockpit dashboard
        ├── style.css           # Glassmorphic cyberpunk styling
        └── app.js              # Live state management, sort debounce, comparator
```
