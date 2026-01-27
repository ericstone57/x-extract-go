const API_BASE = '/api/v1';
let currentCategory = 'download-progress';
let ws = null;
let logEntries = [];
let filteredEntries = [];
let autoScroll = true;
let liveStream = true;
let searchQuery = '';
let levelFilter = '';

// DOM elements
const logContainer = document.getElementById('logEntries');
const searchInput = document.getElementById('searchInput');
const searchBtn = document.getElementById('searchBtn');
const clearSearchBtn = document.getElementById('clearSearchBtn');
const autoScrollCheckbox = document.getElementById('autoScroll');
const liveStreamCheckbox = document.getElementById('liveStream');
const levelFilterSelect = document.getElementById('levelFilter');
const refreshBtn = document.getElementById('refreshBtn');
const exportBtn = document.getElementById('exportBtn');
const clearBtn = document.getElementById('clearBtn');
const logCountSpan = document.getElementById('logCount');
const connectionStatus = document.getElementById('connectionStatus');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    // Don't call loadLogs() here - WebSocket will stream all logs
    connectWebSocket();
});

// Setup event listeners
function setupEventListeners() {
    // Tab switching
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            currentCategory = tab.dataset.category;
            logEntries = [];
            filteredEntries = [];
            // Clear display and reconnect WebSocket
            const container = document.getElementById('logEntries');
            container.innerHTML = '<div class="empty-state">Waiting for logs...</div>';
            if (liveStream) {
                connectWebSocket();
            }
        });
    });

    // Search
    searchBtn.addEventListener('click', performSearch);
    searchInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') performSearch();
    });
    clearSearchBtn.addEventListener('click', clearSearch);

    // Filters
    autoScrollCheckbox.addEventListener('change', (e) => {
        autoScroll = e.target.checked;
    });

    liveStreamCheckbox.addEventListener('change', (e) => {
        liveStream = e.target.checked;
        if (liveStream) {
            connectWebSocket();
        } else {
            disconnectWebSocket();
        }
    });

    levelFilterSelect.addEventListener('change', (e) => {
        levelFilter = e.target.value;
        applyFilters();
    });

    // Actions
    refreshBtn.addEventListener('click', loadLogs);
    exportBtn.addEventListener('click', exportLogs);
    clearBtn.addEventListener('click', clearDisplay);
}

// Load logs from API
async function loadLogs() {
    try {
        const response = await fetch(`${API_BASE}/logs/${currentCategory}?limit=200`);
        if (response.ok) {
            const data = await response.json();
            logEntries = data.entries || [];
            applyFilters();
        }
    } catch (error) {
        console.error('Failed to load logs:', error);
    }
}

// Perform search
async function performSearch() {
    searchQuery = searchInput.value.trim();
    if (!searchQuery) {
        applyFilters();
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/logs/${currentCategory}/search?q=${encodeURIComponent(searchQuery)}&limit=200`);
        if (response.ok) {
            const data = await response.json();
            logEntries = data.entries || [];
            applyFilters();
        }
    } catch (error) {
        console.error('Search failed:', error);
    }
}

// Clear search
function clearSearch() {
    searchInput.value = '';
    searchQuery = '';
    loadLogs();
}

// Apply filters
function applyFilters() {
    filteredEntries = logEntries.filter(entry => {
        if (levelFilter && entry.level !== levelFilter) {
            return false;
        }
        return true;
    });

    renderLogs();
}

// Render logs
function renderLogs() {
    if (filteredEntries.length === 0) {
        logContainer.innerHTML = '<div class="empty-state">No log entries found</div>';
        logCountSpan.textContent = '0 entries';
        return;
    }

    logContainer.innerHTML = filteredEntries.map(entry => createLogEntryHTML(entry)).join('');
    logCountSpan.textContent = `${filteredEntries.length} entries`;

    if (autoScroll) {
        logContainer.parentElement.scrollTop = logContainer.parentElement.scrollHeight;
    }
}

// Create log entry HTML
function createLogEntryHTML(entry) {
    const levelClass = `log-level-${entry.level || 'info'}`;
    const fields = entry.fields ? Object.entries(entry.fields)
        .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
        .join(' ') : '';

    // Check if this is a download progress entry with percent
    let progressHTML = '';
    if (entry.category === 'download-progress' && entry.percent !== undefined) {
        const percent = Math.round(entry.percent * 10) / 10; // Round to 1 decimal
        progressHTML = `
            <div class="progress-bar-container">
                <div class="progress-bar" style="width: ${percent}%"></div>
            </div>
            <div class="progress-text">
                <span>${percent}%</span>
                ${entry.speed ? `<span class="progress-speed">${entry.speed}</span>` : ''}
                ${entry.eta ? `<span class="progress-eta">ETA: ${entry.eta}</span>` : ''}
            </div>
        `;
    }

    return `
        <div class="log-entry">
            <span class="log-timestamp">${entry.timestamp || ''}</span>
            <span class="log-level ${levelClass}">${(entry.level || 'info').toUpperCase()}</span>
            <div class="log-message">
                ${escapeHtml(entry.message || '')}
                ${progressHTML}
                ${fields ? `<div class="log-fields">${escapeHtml(fields)}</div>` : ''}
            </div>
        </div>
    `;
}

// WebSocket connection
function connectWebSocket() {
    disconnectWebSocket();

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}${API_BASE}/logs/stream?category=${currentCategory}`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        connectionStatus.textContent = '● Connected';
        connectionStatus.className = 'status-connected';
    };

    ws.onmessage = (event) => {
        try {
            const entry = JSON.parse(event.data);
            
            // Check for duplicates - don't add if we already have this timestamp
            const isDuplicate = logEntries.some(e => e.timestamp === entry.timestamp && e.message === entry.message);
            if (isDuplicate) {
                return;
            }
            
            logEntries.push(entry);

            // Keep only last 500 entries in memory
            if (logEntries.length > 500) {
                logEntries.shift();
            }

            applyFilters();
        } catch (error) {
            console.error('Failed to parse log entry:', error);
        }
    };

    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        connectionStatus.textContent = '● Error';
        connectionStatus.className = 'status-disconnected';
    };

    ws.onclose = () => {
        connectionStatus.textContent = '● Disconnected';
        connectionStatus.className = 'status-disconnected';

        // Reconnect after 5 seconds if live stream is enabled and page is visible
        if (liveStream && document.visibilityState === 'visible') {
            setTimeout(connectWebSocket, 5000);
        }
    };
}

function disconnectWebSocket() {
    if (ws) {
        ws.close();
        ws = null;
    }
}

// Export logs
function exportLogs() {
    const url = `${API_BASE}/logs/${currentCategory}/export`;
    window.open(url, '_blank');
}

// Clear display
function clearDisplay() {
    logEntries = [];
    filteredEntries = [];
    renderLogs();
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    disconnectWebSocket();
});

