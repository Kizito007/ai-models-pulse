// AI Models Pulse Frontend Application - 100% Live Internet Data Engine

const state = {
  activeTab: 'pulse',
  searchQuery: '',
  category: 'all',
  modelTag: 'all',
  source: 'all',
  timeRange: 'all',
  feedSort: 'latest',
  papersOnly: false,
  layout: 'grid', // 'grid' or 'list'
  offset: 0,
  limit: 24,
  feedItems: [],
  totalItems: 0,
  
  // Live Models State
  models: [],
  radarSort: 'likes',
  radarType: 'all',
  radarModality: 'all',
  radarLab: 'all',
  radarSearch: '',
  
  bookmarks: JSON.parse(localStorage.getItem('ai_pulse_bookmarks') || '[]'),
  searchTimeout: null,
  radarTimeout: null,
};

// Initialize Application
document.addEventListener('DOMContentLoaded', () => {
  initIcons();
  updateBookmarkCount();
  setupSearchListener();
  fetchStats();
  fetchFeed(false);
  fetchModels();
  fetchLatestDrops();
  initLiveStream();
});

function initIcons() {
  if (window.lucide) {
    window.lucide.createIcons();
  }
}

// ================= NAVIGATION & TABS =================
function switchTab(tabId) {
  state.activeTab = tabId;
  const tabs = ['pulse', 'radar', 'compare', 'bookmarks'];
  
  tabs.forEach(t => {
    const btn = document.getElementById(`tab-${t}`);
    const view = document.getElementById(`view-${t}`);
    if (btn && view) {
      if (t === tabId) {
        btn.classList.add('active');
        view.classList.remove('hidden');
      } else {
        btn.classList.remove('active');
        view.classList.add('hidden');
      }
    }
  });

  if (tabId === 'bookmarks') {
    renderBookmarksView();
  } else if (tabId === 'compare') {
    populateComparisonDropdowns();
    updateComparison();
  } else if (tabId === 'radar') {
    fetchModels();
  }

  initIcons();
}

// ================= DATA FETCHING =================
async function fetchStats() {
  try {
    const res = await fetch('/api/stats');
    if (!res.ok) return;
    const stats = await res.json();
    
    document.getElementById('stat-articles').textContent = stats.totalArticles;
    document.getElementById('stat-papers').textContent = stats.totalPapers;
    document.getElementById('stat-models').textContent = stats.totalModels;
    document.getElementById('last-updated-label').textContent = `Live: ${stats.lastUpdatedStr}`;

    renderTrendingTags(stats.trendingModels);
  } catch (err) {
    console.error('Failed to fetch stats:', err);
  }
}

function renderTrendingTags(trending) {
  const container = document.getElementById('trending-tags-list');
  if (!container || !trending) return;

  const entries = Object.entries(trending)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 6);

  if (entries.length === 0) {
    container.innerHTML = '<span class="text-slate-600 font-mono">No trending data</span>';
    return;
  }

  container.innerHTML = entries.map(([tag, count]) => {
    return `
      <button onclick="setModelTag('${tag}')" class="px-2 py-0.5 rounded-full bg-slate-800/80 hover:bg-slate-700 border border-slate-700/60 text-slate-300 font-mono text-[11px] flex items-center space-x-1 transition">
        <span>#${tag}</span>
        <span class="text-teal-400 font-semibold">${count}</span>
      </button>
    `;
  }).join('');
}

// ================= LIVE FEED FETCHING =================
async function fetchFeed(append = false) {
  const statusEl = document.getElementById('feed-status');
  const emptyEl = document.getElementById('feed-empty');
  const loadMoreBtn = document.getElementById('load-more-btn');
  const countEl = document.getElementById('active-results-count');

  if (!append) {
    state.offset = 0;
    statusEl.classList.remove('hidden');
    emptyEl.classList.add('hidden');
    document.getElementById('feed-container').innerHTML = '';
  }

  try {
    const params = new URLSearchParams({
      limit: state.limit,
      offset: state.offset,
      sort: state.feedSort,
    });

    if (state.searchQuery) params.set('q', state.searchQuery);
    if (state.category && state.category !== 'all') params.set('category', state.category);
    if (state.modelTag && state.modelTag !== 'all') params.set('tag', state.modelTag);
    if (state.source && state.source !== 'all') params.set('source', state.source);
    if (state.timeRange && state.timeRange !== 'all') params.set('time', state.timeRange);
    if (state.papersOnly) params.set('papers', 'true');

    const res = await fetch(`/api/feed?${params.toString()}`);
    if (!res.ok) throw new Error('API network response error');
    const data = await res.json();

    statusEl.classList.add('hidden');

    if (append) {
      state.feedItems = [...state.feedItems, ...(data.items || [])];
    } else {
      state.feedItems = data.items || [];
    }
    state.totalItems = data.total;

    if (countEl) {
      countEl.textContent = `${state.totalItems} live matches`;
    }

    if (state.feedItems.length === 0) {
      emptyEl.classList.remove('hidden');
      loadMoreBtn.classList.add('hidden');
    } else {
      emptyEl.classList.add('hidden');
      renderFeedCards(append);
      
      if (state.feedItems.length < state.totalItems) {
        loadMoreBtn.classList.remove('hidden');
      } else {
        loadMoreBtn.classList.add('hidden');
      }
    }
  } catch (err) {
    statusEl.classList.add('hidden');
    console.error('Error fetching live feed:', err);
    showToast('Failed to load live feed from internet.', 'alert-triangle');
  }
}

// ================= LIVE MODELS FETCHING =================
async function fetchModels() {
  try {
    const params = new URLSearchParams({
      sort: state.radarSort,
    });
    if (state.radarType && state.radarType !== 'all') params.set('type', state.radarType);
    if (state.radarModality && state.radarModality !== 'all') params.set('modality', state.radarModality);
    if (state.radarLab && state.radarLab !== 'all') params.set('provider', state.radarLab);
    if (state.radarSearch) params.set('q', state.radarSearch);

    const res = await fetch(`/api/models?${params.toString()}`);
    if (!res.ok) return;
    state.models = await res.json();
    renderRadarModels();
    populateComparisonDropdowns();
  } catch (err) {
    console.error('Error fetching live models:', err);
  }
}

// ================= REAL-TIME SSE MODEL DROPS STREAM =================
async function fetchLatestDrops() {
  try {
    const res = await fetch('/api/models/latest-drops');
    if (!res.ok) return;
    const drops = await res.json();
    renderLiveDrops(drops);
  } catch (err) {
    console.error('Error fetching latest drops:', err);
  }
}

function renderLiveDrops(drops) {
  const container = document.getElementById('live-drops-strip');
  if (!container || !drops || drops.length === 0) return;

  container.innerHTML = drops.slice(0, 12).map(m => createDropCardHTML(m)).join('');
  initIcons();
}

function addLiveDrop(model) {
  const container = document.getElementById('live-drops-strip');
  if (!container) return;

  // Remove placeholder if present
  if (container.textContent.includes('Connecting')) {
    container.innerHTML = '';
  }

  const dropHtml = createDropCardHTML(model, true);
  container.insertAdjacentHTML('afterbegin', dropHtml);

  // Keep max 15 cards
  while (container.children.length > 15) {
    container.removeChild(container.lastChild);
  }

  showToast(`⚡ Real-time Drop: ${model.name} (${model.provider})`, 'zap');
  initIcons();
}

function createDropCardHTML(model, isNew = false) {
  return `
    <div class="shrink-0 bg-slate-950/80 border ${isNew ? 'border-red-500 shadow-lg shadow-red-500/20 animate-pulse' : 'border-slate-800'} rounded-xl px-3 py-2 flex items-center space-x-2.5 transition hover:border-slate-700">
      <div class="w-2 h-2 rounded-full bg-red-400 animate-ping"></div>
      <div class="flex flex-col">
        <div class="flex items-center space-x-1.5">
          <span class="font-semibold text-white text-xs truncate max-w-[140px]" title="${escapeHtml(model.name)}">${escapeHtml(model.name)}</span>
          <span class="text-[10px] font-mono text-slate-400">by ${escapeHtml(model.provider)}</span>
        </div>
        <div class="flex items-center space-x-2 text-[10px] text-slate-500 font-mono mt-0.5">
          <span class="text-teal-400">${escapeHtml(model.modality || 'Text')}</span>
          <span>•</span>
          <span class="text-slate-400">${model.releaseDate || 'Just now'}</span>
          <button onclick="quickCompare('${model.id}')" class="text-teal-300 hover:underline">Compare</button>
        </div>
      </div>
    </div>
  `;
}

function initLiveStream() {
  if (!window.EventSource) return;

  try {
    const evtSource = new EventSource('/api/models/live-stream');

    evtSource.addEventListener('model_drop', (e) => {
      try {
        const model = JSON.parse(e.data);
        addLiveDrop(model);
      } catch (err) {}
    });

    evtSource.onerror = () => {
      const statusEl = document.getElementById('sse-status');
      if (statusEl) {
        statusEl.innerHTML = '<span class="text-amber-400 font-mono text-[10px]">Reconnecting stream...</span>';
      }
    };

    evtSource.onopen = () => {
      const statusEl = document.getElementById('sse-status');
      if (statusEl) {
        statusEl.innerHTML = '<span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span> Real-time Stream Active';
      }
    };
  } catch (err) {
    console.error('SSE connection error:', err);
  }
}

// ================= RENDERING FEED ITEMS =================
function renderFeedCards(append = false) {
  const container = document.getElementById('feed-container');
  if (!container) return;

  const html = state.feedItems.map(item => createCardHTML(item)).join('');

  if (append) {
    container.insertAdjacentHTML('beforeend', html);
  } else {
    container.innerHTML = html;
  }

  initIcons();
}

function createCardHTML(item) {
  const isBookmarked = state.bookmarks.some(b => b.id === item.id);
  
  // Model tags badges
  const tagsHtml = (item.modelTags || []).map(tag => {
    let colorClass = 'bg-teal-500/10 text-teal-300 border-teal-500/30';
    if (tag.includes('DeepSeek')) colorClass = 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30';
    if (tag.includes('Gemini')) colorClass = 'bg-blue-500/10 text-blue-300 border-blue-500/30';
    if (tag.includes('Claude')) colorClass = 'bg-amber-500/10 text-amber-300 border-amber-500/30';
    if (tag.includes('GPT')) colorClass = 'bg-teal-500/10 text-teal-300 border-teal-500/30';
    if (tag.includes('Llama')) colorClass = 'bg-purple-500/10 text-purple-300 border-purple-500/30';
    if (tag.includes('Qwen')) colorClass = 'bg-rose-500/10 text-rose-300 border-rose-500/30';
    if (tag.includes('Flux')) colorClass = 'bg-orange-500/10 text-orange-300 border-orange-500/30';

    return `<span class="px-2 py-0.5 rounded text-[11px] font-mono border ${colorClass} font-medium">#${escapeHtml(tag)}</span>`;
  }).join('');

  return `
    <article class="feed-card bg-slate-900/50 border border-slate-800 rounded-2xl p-4 flex flex-col justify-between backdrop-blur transition relative group">
      <div>
        <!-- Top bar: Source + Time -->
        <div class="flex items-center justify-between gap-2 mb-3">
          <div class="flex items-center space-x-1.5 overflow-hidden">
            <span class="text-base">${item.sourceIcon || '⚡'}</span>
            <span class="text-xs font-semibold text-slate-300 truncate">${escapeHtml(item.source)}</span>
            ${item.isPaper ? '<span class="text-[10px] px-1.5 py-0.2 bg-cyan-950/60 text-cyan-400 border border-cyan-800/60 rounded font-mono">PAPER</span>' : ''}
          </div>
          <span class="text-[11px] font-mono text-slate-500 shrink-0">${item.timeAgo}</span>
        </div>

        <!-- Title -->
        <h3 class="text-sm font-semibold text-white group-hover:text-teal-300 transition line-clamp-2 leading-snug mb-2">
          <a href="${escapeHtml(item.link)}" target="_blank" rel="noopener noreferrer">${escapeHtml(item.title)}</a>
        </h3>

        <!-- Summary -->
        <p class="text-xs text-slate-400 line-clamp-3 leading-relaxed mb-4">
          ${escapeHtml(item.summary)}
        </p>
      </div>

      <div>
        <!-- Tags -->
        <div class="flex flex-wrap gap-1.5 mb-3">
          ${tagsHtml}
        </div>

        <!-- Footer Strip -->
        <div class="pt-3 border-t border-slate-800/60 flex items-center justify-between text-xs text-slate-500">
          <div class="flex items-center space-x-2">
            ${item.upvotes ? `
              <span class="flex items-center space-x-1 text-emerald-400 font-mono text-[11px]">
                <i data-lucide="arrow-big-up" class="w-3.5 h-3.5"></i>
                <span>${item.upvotes}</span>
              </span>` : ''}
            ${item.author ? `<span class="truncate max-w-[120px] text-slate-500 text-[11px]">${escapeHtml(item.author)}</span>` : ''}
          </div>

          <div class="flex items-center space-x-1">
            <button onclick="openModal('${item.id}')" title="Quick Abstract Preview" class="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-white transition">
              <i data-lucide="eye" class="w-4 h-4"></i>
            </button>
            <button onclick="toggleBookmark('${item.id}')" title="${isBookmarked ? 'Remove Bookmark' : 'Bookmark'}" class="p-1.5 rounded-lg hover:bg-slate-800 ${isBookmarked ? 'text-teal-400' : 'text-slate-400 hover:text-white'} transition">
              <i data-lucide="${isBookmarked ? 'bookmark-check' : 'bookmark'}" class="w-4 h-4"></i>
            </button>
            <a href="${escapeHtml(item.link)}" target="_blank" rel="noopener noreferrer" title="Open Source" class="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-teal-400 transition">
              <i data-lucide="external-link" class="w-4 h-4"></i>
            </a>
          </div>
        </div>
      </div>
    </article>
  `;
}

// ================= RENDERING LIVE MODEL RADAR =================
function renderRadarModels() {
  const container = document.getElementById('models-radar-grid');
  if (!container) return;

  if (state.models.length === 0) {
    container.innerHTML = `
      <div class="col-span-2 text-center py-12 bg-slate-900/30 border border-slate-800 rounded-2xl">
        <i data-lucide="cpu" class="w-8 h-8 mx-auto text-slate-600 mb-2"></i>
        <p class="text-sm text-slate-400">Syncing live models from Hugging Face API...</p>
      </div>
    `;
    initIcons();
    return;
  }

  container.innerHTML = state.models.map(model => {
    // Benchmarks grid if present
    const benchEntries = Object.entries(model.benchmarks || {});
    const benchHtml = benchEntries.map(([name, val]) => `
      <div class="bg-slate-950/60 border border-slate-800/80 rounded-xl p-2.5">
        <div class="text-[10px] font-mono uppercase text-slate-400 truncate">${escapeHtml(name)}</div>
        <div class="text-sm font-mono font-bold text-teal-400 mt-0.5">${escapeHtml(val)}</div>
      </div>
    `).join('');

    // Feature bullets
    const featHtml = (model.keyFeatures || []).map(f => `
      <li class="flex items-start space-x-1.5 text-xs text-slate-300">
        <i data-lucide="check" class="w-3.5 h-3.5 text-teal-400 shrink-0 mt-0.5"></i>
        <span>${escapeHtml(f)}</span>
      </li>
    `).join('');

    // Direct Links
    const linksHtml = Object.entries(model.links || {}).map(([key, url]) => `
      <a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer" class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-mono font-medium flex items-center space-x-1 transition">
        <span>${escapeHtml(key)}</span>
        <i data-lucide="external-link" class="w-3 h-3"></i>
      </a>
    `).join('');

    return `
      <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-5 flex flex-col justify-between backdrop-blur space-y-4 hover:border-slate-700 transition">
        <div>
          <!-- Header: Lab + Type + Live Stats -->
          <div class="flex items-start justify-between gap-3">
            <div>
              <div class="flex items-center space-x-2">
                <span class="text-xs font-semibold px-2 py-0.5 rounded-full ${model.type.includes('Open') ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/30'}">
                  ${escapeHtml(model.type)}
                </span>
                <span class="text-xs font-mono text-slate-400 font-medium">${escapeHtml(model.provider)}</span>
                ${model.isLiveHF ? '<span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20">LIVE HF</span>' : ''}
              </div>
              <h3 class="text-xl font-bold text-white mt-1.5 tracking-tight break-all">${escapeHtml(model.name)}</h3>
            </div>
            
            <div class="flex flex-col items-end space-y-1">
              <span class="text-xs font-mono px-2 py-0.5 bg-slate-950 rounded-lg border border-slate-800 text-slate-400 shrink-0">
                ${escapeHtml(model.releaseDate || 'Live')}
              </span>
              <!-- Live Metrics Badges -->
              <div class="flex items-center gap-1.5 font-mono text-[11px]">
                ${model.likes ? `
                  <span class="flex items-center space-x-1 text-rose-400 bg-rose-950/30 border border-rose-900/50 px-2 py-0.5 rounded-full">
                    <i data-lucide="heart" class="w-3 h-3 fill-rose-400"></i>
                    <span>${model.likes.toLocaleString()}</span>
                  </span>
                ` : ''}
                ${model.downloadsStr ? `
                  <span class="flex items-center space-x-1 text-teal-400 bg-teal-950/30 border border-teal-900/50 px-2 py-0.5 rounded-full" title="${model.downloads} downloads">
                    <i data-lucide="download" class="w-3 h-3"></i>
                    <span>${model.downloadsStr}</span>
                  </span>
                ` : ''}
              </div>
            </div>
          </div>

          <p class="text-xs text-slate-300 mt-2.5 leading-relaxed">
            ${escapeHtml(model.description)}
          </p>

          <!-- Specifications Row -->
          <div class="grid grid-cols-3 gap-2 mt-4 pt-4 border-t border-slate-800/80 text-xs">
            <div>
              <span class="text-[10px] uppercase font-mono text-slate-500 block">Parameters</span>
              <span class="font-mono font-semibold text-slate-200">${escapeHtml(model.parameters)}</span>
            </div>
            <div>
              <span class="text-[10px] uppercase font-mono text-slate-500 block">Task / Modality</span>
              <span class="font-mono font-semibold text-teal-400 truncate block">${escapeHtml(model.modality)}</span>
            </div>
            <div>
              <span class="text-[10px] uppercase font-mono text-slate-500 block">License</span>
              <span class="font-mono font-semibold text-slate-200 truncate block">${escapeHtml(model.license)}</span>
            </div>
          </div>

          <!-- Benchmarks if available -->
          ${benchEntries.length > 0 ? `
            <div class="mt-4">
              <div class="text-[11px] font-mono uppercase text-slate-400 mb-2">Benchmark Scores:</div>
              <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
                ${benchHtml}
              </div>
            </div>
          ` : ''}

          <!-- Highlights if available -->
          ${(model.keyFeatures || []).length > 0 ? `
            <div class="mt-4">
              <div class="text-[11px] font-mono uppercase text-slate-400 mb-2">Architectural Highlights:</div>
              <ul class="space-y-1.5">
                ${featHtml}
              </ul>
            </div>
          ` : ''}
        </div>

        <!-- Links Row & Quick Compare Action -->
        <div class="pt-3 border-t border-slate-800/80 flex items-center justify-between">
          <button onclick="quickCompare('${model.id}')" class="px-2.5 py-1 bg-teal-500/10 hover:bg-teal-500/20 text-teal-300 border border-teal-500/30 rounded-lg text-xs font-mono font-medium flex items-center space-x-1 transition">
            <i data-lucide="scale" class="w-3.5 h-3.5"></i>
            <span>Compare Live</span>
          </button>
          
          <div class="flex items-center space-x-2">
            ${linksHtml}
          </div>
        </div>
      </div>
    `;
  }).join('');

  initIcons();
}

function onRadarSortChange(val) {
  state.radarSort = val;
  fetchModels();
}

function onRadarSearch(val) {
  state.radarSearch = val.trim();
  clearTimeout(state.radarTimeout);
  state.radarTimeout = setTimeout(() => {
    fetchModels();
  }, 250);
}

function filterRadarType(type) {
  state.radarType = type;
  document.querySelectorAll('.radar-type-btn').forEach(btn => {
    btn.classList.toggle('active', btn.textContent.toLowerCase().includes(type) || (type === 'all' && btn.textContent.includes('All')));
  });
  fetchModels();
}

function filterRadarModality(mod) {
  state.radarModality = mod;
  document.querySelectorAll('.radar-modality-btn').forEach(btn => {
    btn.classList.toggle('active', btn.textContent.toLowerCase().includes(mod));
  });
  fetchModels();
}

function filterRadarLab(lab) {
  state.radarLab = lab;
  document.querySelectorAll('.radar-lab-btn').forEach(btn => {
    const fnAttr = btn.getAttribute('onclick') || '';
    btn.classList.toggle('active', fnAttr.includes(`'${lab}'`));
  });
  fetchModels();
}

// ================= LIVE MODEL COMPARISON VIEW =================
function populateComparisonDropdowns() {
  const selects = [
    document.getElementById('compare-select-1'),
    document.getElementById('compare-select-2'),
    document.getElementById('compare-select-3')
  ];

  if (!selects[0] || state.models.length === 0) return;

  const defaultIds = [
    state.models[0]?.id || 'deepseek-ai/DeepSeek-R1',
    state.models[1]?.id || 'claude-3-5-sonnet',
    state.models[2]?.id || 'gemini-2-flash'
  ];

  selects.forEach((select, idx) => {
    const currentVal = select.value;
    select.innerHTML = state.models.map(m => `
      <option value="${m.id}" ${currentVal ? (m.id === currentVal ? 'selected' : '') : (m.id === defaultIds[idx] ? 'selected' : '')}>
        ${escapeHtml(m.name)} — ${escapeHtml(m.provider)} (${m.downloadsStr ? m.downloadsStr + ' DLs' : m.type})
      </option>
    `).join('');
  });
}

function quickCompare(modelId) {
  switchTab('compare');
  const sel1 = document.getElementById('compare-select-1');
  if (sel1) {
    sel1.value = modelId;
    updateComparison();
  }
}

function updateComparison() {
  const s1 = document.getElementById('compare-select-1');
  const s2 = document.getElementById('compare-select-2');
  const s3 = document.getElementById('compare-select-3');

  if (!s1 || !s2 || !s3) return;

  const id1 = s1.value;
  const id2 = s2.value;
  const id3 = s3.value;

  const m1 = state.models.find(m => m.id === id1) || state.models[0];
  const m2 = state.models.find(m => m.id === id2) || state.models[1];
  const m3 = state.models.find(m => m.id === id3) || state.models[2];

  if (!m1 || !m2 || !m3) return;

  document.getElementById('comp-header-1').textContent = m1.name;
  document.getElementById('comp-header-2').textContent = m2.name;
  document.getElementById('comp-header-3').textContent = m3.name;

  const rows = [
    { label: 'Provider / Lab', v1: m1.provider, v2: m2.provider, v3: m3.provider },
    { label: 'Live Community Stars (Likes)', v1: `❤️ ${m1.likes ? m1.likes.toLocaleString() : 'N/A'}`, v2: `❤️ ${m2.likes ? m2.likes.toLocaleString() : 'N/A'}`, v3: `❤️ ${m3.likes ? m3.likes.toLocaleString() : 'N/A'}`, highlight: true },
    { label: 'Live Monthly Downloads', v1: `📥 ${m1.downloadsStr || 'API/Private'}`, v2: `📥 ${m2.downloadsStr || 'API/Private'}`, v3: `📥 ${m3.downloadsStr || 'API/Private'}`, highlight: true },
    { label: 'Deployment Model', v1: m1.type, v2: m2.type, v3: m3.type },
    { label: 'Task Modality', v1: m1.modality, v2: m2.modality, v3: m3.modality },
    { label: 'Parameters', v1: m1.parameters, v2: m2.parameters, v3: m3.parameters },
    { label: 'License', v1: m1.license, v2: m2.license, v3: m3.license },
    { label: 'ArXiv Paper', v1: m1.arxivID ? `arXiv:${m1.arxivID}` : '—', v2: m2.arxivID ? `arXiv:${m2.arxivID}` : '—', v3: m3.arxivID ? `arXiv:${m3.arxivID}` : '—' },
    { label: 'Release / Modified', v1: m1.releaseDate || 'Recent', v2: m2.releaseDate || 'Recent', v3: m3.releaseDate || 'Recent' },
  ];

  // Benchmarks union if present
  const benchKeys = Array.from(new Set([
    ...Object.keys(m1.benchmarks || {}),
    ...Object.keys(m2.benchmarks || {}),
    ...Object.keys(m3.benchmarks || {}),
  ]));

  benchKeys.forEach(k => {
    rows.push({
      label: `Eval: ${k}`,
      v1: (m1.benchmarks && m1.benchmarks[k]) || '—',
      v2: (m2.benchmarks && m2.benchmarks[k]) || '—',
      v3: (m3.benchmarks && m3.benchmarks[k]) || '—',
      bench: true
    });
  });

  const tbody = document.getElementById('comparison-body');
  tbody.innerHTML = rows.map(r => `
    <tr class="hover:bg-slate-900/50 transition">
      <td class="py-3 px-4 font-medium text-slate-400">${escapeHtml(r.label)}</td>
      <td class="py-3 px-4 ${r.highlight ? 'text-teal-400 font-bold' : r.bench ? 'text-emerald-300' : 'text-slate-200'}">${escapeHtml(r.v1)}</td>
      <td class="py-3 px-4 ${r.highlight ? 'text-cyan-400 font-bold' : r.bench ? 'text-emerald-300' : 'text-slate-200'}">${escapeHtml(r.v2)}</td>
      <td class="py-3 px-4 ${r.highlight ? 'text-indigo-400 font-bold' : r.bench ? 'text-emerald-300' : 'text-slate-200'}">${escapeHtml(r.v3)}</td>
    </tr>
  `).join('');
}

// ================= BOOKMARKS VIEW =================
function renderBookmarksView() {
  const container = document.getElementById('bookmarks-container');
  const emptyEl = document.getElementById('bookmarks-empty');

  if (state.bookmarks.length === 0) {
    emptyEl.classList.remove('hidden');
    container.innerHTML = '';
  } else {
    emptyEl.classList.add('hidden');
    container.innerHTML = state.bookmarks.map(item => createCardHTML(item)).join('');
  }

  initIcons();
}

function toggleBookmark(itemId) {
  const idx = state.bookmarks.findIndex(b => b.id === itemId);
  if (idx >= 0) {
    state.bookmarks.splice(idx, 1);
    showToast('Removed from saved bookmarks', 'bookmark-minus');
  } else {
    const item = state.feedItems.find(i => i.id === itemId);
    if (item) {
      state.bookmarks.push(item);
      showToast('Saved to bookmarks', 'bookmark-check');
    }
  }

  localStorage.setItem('ai_pulse_bookmarks', JSON.stringify(state.bookmarks));
  updateBookmarkCount();

  if (state.activeTab === 'bookmarks') {
    renderBookmarksView();
  } else {
    renderFeedCards();
  }
}

function updateBookmarkCount() {
  const countEl = document.getElementById('bookmark-count');
  if (countEl) countEl.textContent = state.bookmarks.length;
}

function clearAllBookmarks() {
  if (state.bookmarks.length === 0) return;
  if (confirm('Clear all saved bookmarks?')) {
    state.bookmarks = [];
    localStorage.removeItem('ai_pulse_bookmarks');
    updateBookmarkCount();
    renderBookmarksView();
    showToast('Cleared all bookmarks', 'trash');
  }
}

// ================= CONTROLS & FILTERS =================
function setupSearchListener() {
  const input = document.getElementById('feed-search');
  const clearBtn = document.getElementById('clear-search-btn');

  input.addEventListener('input', (e) => {
    state.searchQuery = e.target.value.trim();
    if (state.searchQuery) {
      clearBtn.classList.remove('hidden');
    } else {
      clearBtn.classList.add('hidden');
    }

    clearTimeout(state.searchTimeout);
    state.searchTimeout = setTimeout(() => {
      fetchFeed(false);
    }, 280);
  });
}

function clearSearch() {
  const input = document.getElementById('feed-search');
  input.value = '';
  state.searchQuery = '';
  document.getElementById('clear-search-btn').classList.add('hidden');
  fetchFeed(false);
}

function onSortChange(val) {
  state.feedSort = val;
  fetchFeed(false);
}

function onTimeChange(val) {
  state.timeRange = val;
  fetchFeed(false);
}

function onSourceChange(val) {
  state.source = val;
  fetchFeed(false);
}

function setCategory(cat) {
  state.category = cat;
  document.querySelectorAll('.cat-pill').forEach(btn => {
    btn.classList.toggle('active', btn.textContent.includes(cat) || (cat === 'all' && btn.textContent === 'All'));
  });
  fetchFeed(false);
}

function setModelTag(tag) {
  state.modelTag = tag;
  document.querySelectorAll('.tag-pill').forEach(btn => {
    btn.classList.toggle('active', btn.textContent.includes(tag) || (tag === 'all' && btn.textContent.includes('All')));
  });
  if (state.activeTab !== 'pulse') {
    switchTab('pulse');
  }
  fetchFeed(false);
}

function togglePapersOnly() {
  state.papersOnly = !state.papersOnly;
  const btn = document.getElementById('toggle-papers');
  if (state.papersOnly) {
    btn.classList.add('bg-teal-950', 'border-teal-500', 'text-teal-300');
    btn.classList.remove('bg-slate-950/60', 'border-slate-800', 'text-slate-300');
  } else {
    btn.classList.remove('bg-teal-950', 'border-teal-500', 'text-teal-300');
    btn.classList.add('bg-slate-950/60', 'border-slate-800', 'text-slate-300');
  }
  fetchFeed(false);
}

function setLayout(mode) {
  state.layout = mode;
  const container = document.getElementById('feed-container');
  const btnGrid = document.getElementById('btn-layout-grid');
  const btnList = document.getElementById('btn-layout-list');

  if (mode === 'list') {
    container.classList.remove('grid', 'grid-cols-1', 'md:grid-cols-2', 'lg:grid-cols-3');
    container.classList.add('layout-list');
    btnList.classList.add('text-teal-400', 'bg-slate-800');
    btnList.classList.remove('text-slate-400');
    btnGrid.classList.remove('text-teal-400', 'bg-slate-800');
    btnGrid.classList.add('text-slate-400');
  } else {
    container.classList.add('grid', 'grid-cols-1', 'md:grid-cols-2', 'lg:grid-cols-3');
    container.classList.remove('layout-list');
    btnGrid.classList.add('text-teal-400', 'bg-slate-800');
    btnGrid.classList.remove('text-slate-400');
    btnList.classList.remove('text-teal-400', 'bg-slate-800');
    btnList.classList.add('text-slate-400');
  }
}

function resetFilters() {
  state.searchQuery = '';
  state.category = 'all';
  state.modelTag = 'all';
  state.source = 'all';
  state.timeRange = 'all';
  state.feedSort = 'latest';
  state.papersOnly = false;
  
  document.getElementById('feed-search').value = '';
  document.getElementById('clear-search-btn').classList.add('hidden');
  document.getElementById('feed-sort-select').value = 'latest';
  document.getElementById('feed-time-select').value = 'all';
  document.getElementById('feed-source-select').value = 'all';
  
  document.querySelectorAll('.cat-pill').forEach(btn => btn.classList.toggle('active', btn.textContent === 'All'));
  document.querySelectorAll('.tag-pill').forEach(btn => btn.classList.toggle('active', btn.textContent.includes('All')));
  
  const pBtn = document.getElementById('toggle-papers');
  pBtn.classList.remove('bg-teal-950', 'border-teal-500', 'text-teal-300');
  pBtn.classList.add('bg-slate-950/60', 'border-slate-800', 'text-slate-300');

  fetchFeed(false);
}

function loadMore() {
  state.offset += state.limit;
  fetchFeed(true);
}

// ================= REFRESH LIVE DATA =================
async function triggerManualRefresh() {
  const icon = document.getElementById('refresh-icon');
  icon.classList.add('animate-spin');

  try {
    const res = await fetch('/api/refresh', { method: 'POST' });
    if (!res.ok) throw new Error('Refresh failed');
    const data = await res.json();
    
    showToast(`Live sync complete! ${data.totalArticles} articles & ${data.totalModels} models live.`, 'check-circle');
    fetchStats();
    fetchFeed(false);
    fetchModels();
  } catch (err) {
    showToast('Failed to sync live data.', 'alert-triangle');
  } finally {
    setTimeout(() => {
      icon.classList.remove('animate-spin');
    }, 600);
  }
}

// ================= MODAL ABSTRACT PREVIEW =================
function openModal(itemId) {
  const item = state.feedItems.find(i => i.id === itemId) || state.bookmarks.find(i => i.id === itemId);
  if (!item) return;

  const modal = document.getElementById('modal-container');
  const content = document.getElementById('modal-content');

  content.innerHTML = `
    <div>
      <div class="flex items-center space-x-2 text-xs font-mono text-slate-400 mb-2">
        <span>${item.sourceIcon || '⚡'} ${escapeHtml(item.source)}</span>
        <span>•</span>
        <span>${item.timeAgo}</span>
        ${item.isPaper ? '<span class="text-[10px] px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800">RESEARCH PAPER</span>' : ''}
      </div>
      <h2 class="text-xl font-bold text-white tracking-tight leading-snug">${escapeHtml(item.title)}</h2>
      ${item.author ? `<p class="text-xs text-slate-400 mt-1">Authors: <span class="text-slate-200">${escapeHtml(item.author)}</span></p>` : ''}
    </div>

    ${item.thumbnail ? `
      <div class="rounded-xl overflow-hidden border border-slate-800 max-h-60 bg-slate-950">
        <img src="${escapeHtml(item.thumbnail)}" alt="Thumbnail" class="w-full object-cover">
      </div>
    ` : ''}

    <div class="space-y-2">
      <h4 class="text-xs font-mono uppercase text-slate-400">Summary / Abstract:</h4>
      <p class="text-sm text-slate-300 leading-relaxed bg-slate-950/70 p-4 rounded-xl border border-slate-800/80 font-sans whitespace-pre-line">
        ${escapeHtml(item.summary)}
      </p>
    </div>

    <!-- Tags -->
    <div class="flex flex-wrap gap-1.5 pt-2">
      ${(item.modelTags || []).map(t => `<span class="px-2 py-0.5 rounded text-xs font-mono bg-teal-500/10 text-teal-300 border border-teal-500/30">#${escapeHtml(t)}</span>`).join('')}
    </div>

    <div class="pt-4 border-t border-slate-800 flex items-center justify-between">
      <button onclick="toggleBookmark('${item.id}'); closeModal();" class="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-700 transition flex items-center space-x-1.5">
        <i data-lucide="bookmark" class="w-4 h-4"></i>
        <span>Bookmark</span>
      </button>

      <a href="${escapeHtml(item.link)}" target="_blank" rel="noopener noreferrer" class="px-5 py-2 rounded-xl bg-teal-500 hover:bg-teal-400 text-black text-xs font-bold transition flex items-center space-x-1.5 shadow-lg shadow-teal-500/20">
        <span>Open Original Publication</span>
        <i data-lucide="external-link" class="w-4 h-4"></i>
      </a>
    </div>
  `;

  modal.classList.remove('hidden');
  initIcons();
}

function closeModal() {
  document.getElementById('modal-container').classList.add('hidden');
}

document.getElementById('modal-container').addEventListener('click', (e) => {
  if (e.target.id === 'modal-container') closeModal();
});

// ================= TOAST NOTIFICATION =================
function showToast(text, iconName = 'check-circle') {
  const toast = document.getElementById('toast');
  const textEl = document.getElementById('toast-text');
  const iconEl = document.getElementById('toast-icon');

  textEl.textContent = text;
  iconEl.setAttribute('data-lucide', iconName);
  initIcons();

  toast.classList.remove('translate-y-20', 'opacity-0');
  toast.classList.add('translate-y-0', 'opacity-100');

  setTimeout(() => {
    toast.classList.remove('translate-y-0', 'opacity-100');
    toast.classList.add('translate-y-20', 'opacity-0');
  }, 2800);
}

// Utility: HTML Escaping
function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}
