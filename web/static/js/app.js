const API_BASE = '/api/v1';

let progressWS = null;
let activeDownloads = new Map();

// Load stats
async function loadStats() {
    try {
        const response = await fetch(`${API_BASE}/downloads/stats`);
        const stats = await response.json();
        
        document.getElementById('stat-total').textContent = stats.total || 0;
        document.getElementById('stat-queued').textContent = stats.queued || 0;
        document.getElementById('stat-processing').textContent = stats.processing || 0;
        document.getElementById('stat-completed').textContent = stats.completed || 0;
        document.getElementById('stat-failed').textContent = stats.failed || 0;
    } catch (error) {
        console.error('Failed to load stats:', error);
    }
}

// Load downloads
async function loadDownloads() {
    const statusFilter = document.getElementById('status-filter').value;
    const url = statusFilter ? `${API_BASE}/downloads?status=${statusFilter}` : `${API_BASE}/downloads`;
    
    try {
        const response = await fetch(url);
        const downloads = await response.json();
        
        const container = document.getElementById('downloads-list');
        
        if (!downloads || downloads.length === 0) {
            container.innerHTML = '<div class="empty-state">No downloads found</div>';
            return;
        }
        
        container.innerHTML = downloads.map(download => `
            <div class="download-item">
                <div class="download-header">
                    <div class="download-url">${truncate(download.url, 60)}</div>
                    <span class="download-status status-${download.status}">${download.status}</span>
                </div>
                <div class="download-meta">
                    <span>📱 ${download.platform}</span>
                    <span>🔧 ${download.mode}</span>
                    <span>🕐 ${new Date(download.created_at).toLocaleString()}</span>
                    ${download.file_path ? `<span>📁 ${download.file_path.split('/').pop()}</span>` : ''}
                </div>
                ${download.error_message ? `<div style="color: #f44336; margin-top: 10px;">❌ ${download.error_message}</div>` : ''}
                <div class="download-actions">
                    ${download.status === 'queued' || download.status === 'processing' ?
                        `<button onclick="cancelDownload('${download.id}')">Cancel</button>` : ''}
                    ${download.status === 'failed' ?
                        `<button onclick="retryDownload('${download.id}')">Retry</button>` : ''}
                    <button onclick="viewDownloadLogs('${download.id}')">📋 View Logs</button>
                </div>
            </div>
        `).join('');

        // Update active downloads for progress tracking
        updateActiveDownloads(downloads);
    } catch (error) {
        console.error('Failed to load downloads:', error);
    }
}

// Update active downloads map and show progress section
function updateActiveDownloads(downloads) {
    const processing = downloads.filter(d => d.status === 'processing');
    
    if (processing.length > 0) {
        document.getElementById('progress-section').style.display = 'block';
        const container = document.getElementById('active-downloads');
        
        container.innerHTML = processing.map(download => `
            <div class="progress-item" id="progress-${download.id}">
                <div class="progress-header">
                    <div class="progress-url">${truncate(download.url, 50)}</div>
                    <span class="progress-platform">${download.platform}</span>
                </div>
                <div class="progress-bar-container">
                    <div class="progress-bar" id="progress-bar-${download.id}" style="width: 0%"></div>
                </div>
                <div class="progress-output" id="progress-output-${download.id}">Connecting...</div>
                <button class="btn-cancel" onclick="cancelDownload('${download.id}')">Cancel</button>
            </div>
        `).join('');

        // Connect to progress WebSocket for each active download
        processing.forEach(download => {
            connectProgressWebSocket(download.id);
        });
    } else {
        document.getElementById('progress-section').style.display = 'none';
    }
}

// Connect to progress WebSocket
function connectProgressWebSocket(downloadId) {
    // Close existing connection for this download
    if (activeDownloads.has(downloadId)) {
        activeDownloads.get(downloadId).close();
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}${API_BASE}/downloads/${downloadId}/progress`;

    const ws = new WebSocket(wsUrl);
    activeDownloads.set(downloadId, ws);

    ws.onopen = () => {
        console.log(`Connected to progress stream for ${downloadId}`);
        updateProgressOutput(downloadId, 'Connected - waiting for progress...', 0);
    };

    ws.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            updateProgress(downloadId, msg);
        } catch (error) {
            console.error('Failed to parse progress message:', error);
        }
    };

    ws.onerror = (error) => {
        console.error(`Progress WebSocket error for ${downloadId}:`, error);
    };

    ws.onclose = () => {
        console.log(`Progress WebSocket closed for ${downloadId}`);
        activeDownloads.delete(downloadId);
    };
}

// Update progress display
function updateProgress(downloadId, msg) {
    const outputElement = document.getElementById(`progress-output-${downloadId}`);
    const barElement = document.getElementById(`progress-bar-${downloadId}`);

    if (outputElement) {
        outputElement.textContent = msg.output || msg.status;
        outputElement.className = `progress-output status-${msg.status}`;
    }

    if (barElement && msg.percent > 0) {
        barElement.style.width = `${Math.min(msg.percent, 100)}%`;
    }
}

// Update progress output text
function updateProgressOutput(downloadId, text, percent) {
    const outputElement = document.getElementById(`progress-output-${downloadId}`);
    const barElement = document.getElementById(`progress-bar-${downloadId}`);

    if (outputElement) {
        outputElement.textContent = text;
    }

    if (barElement && percent > 0) {
        barElement.style.width = `${Math.min(percent, 100)}%`;
    }
}

// Add download
document.getElementById('add-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const url = document.getElementById('url-input').value;
    const mode = document.getElementById('mode-select').value;
    
    try {
        const response = await fetch(`${API_BASE}/downloads`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ url, mode }),
        });
        
        if (response.ok) {
            document.getElementById('url-input').value = '';
            loadStats();
            loadDownloads();
            alert('Download added successfully!');
        } else {
            const error = await response.json();
            alert(`Failed to add download: ${error.error}`);
        }
    } catch (error) {
        console.error('Failed to add download:', error);
        alert('Failed to add download');
    }
});

// Cancel download
async function cancelDownload(id) {
    if (!confirm('Are you sure you want to cancel this download?')) {
        return;
    }
    
    try {
        const response = await fetch(`${API_BASE}/downloads/${id}/cancel`, {
            method: 'POST',
        });
        
        if (response.ok) {
            // Close WebSocket connection
            if (activeDownloads.has(id)) {
                activeDownloads.get(id).close();
                activeDownloads.delete(id);
            }
            loadStats();
            loadDownloads();
        } else {
            alert('Failed to cancel download');
        }
    } catch (error) {
        console.error('Failed to cancel download:', error);
        alert('Failed to cancel download');
    }
}

// Retry download
async function retryDownload(id) {
    try {
        const response = await fetch(`${API_BASE}/downloads/${id}/retry`, {
            method: 'POST',
        });

        if (response.ok) {
            loadStats();
            loadDownloads();
            alert('Download queued for retry');
        } else {
            alert('Failed to retry download');
        }
    } catch (error) {
        console.error('Failed to retry download:', error);
        alert('Failed to retry download');
    }
}

// View download logs
async function viewDownloadLogs(id) {
    window.open(`/download_logs?id=${id}`, '_blank');
}

// View logs (legacy modal)
async function viewLogs(id) {
    try {
        const response = await fetch(`${API_BASE}/downloads/${id}/logs`);

        if (response.ok) {
            const logs = await response.text();
            showLogsModal(id, logs);
        } else {
            alert('Failed to load logs');
        }
    } catch (error) {
        console.error('Failed to load logs:', error);
        alert('Failed to load logs');
    }
}

// Show logs modal
function showLogsModal(id, logs) {
    const modal = document.createElement('div');
    modal.className = 'modal';
    modal.innerHTML = `
        <div class="modal-content">
            <div class="modal-header">
                <h3>Process Logs - ${id}</h3>
                <button class="modal-close" onclick="closeLogsModal()">&times;</button>
            </div>
            <div class="modal-body">
                <pre class="logs-content">${escapeHtml(logs || 'No logs available')}</pre>
            </div>
            <div class="modal-footer">
                <button onclick="copyLogs('${id}')">Copy to Clipboard</button>
                <button onclick="closeLogsModal()">Close</button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);

    // Close on background click
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            closeLogsModal();
        }
    });
}

// Close logs modal
function closeLogsModal() {
    const modal = document.querySelector('.modal');
    if (modal) {
        modal.remove();
    }
}

// Copy logs to clipboard
async function copyLogs(id) {
    try {
        const response = await fetch(`${API_BASE}/downloads/${id}/logs`);
        const logs = await response.text();
        await navigator.clipboard.writeText(logs);
        alert('Logs copied to clipboard!');
    } catch (error) {
        console.error('Failed to copy logs:', error);
        alert('Failed to copy logs');
    }
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Utility function
function truncate(str, maxLen) {
    return str.length > maxLen ? str.substring(0, maxLen) + '...' : str;
}

// Event listeners
document.getElementById('status-filter').addEventListener('change', loadDownloads);
document.getElementById('refresh-btn').addEventListener('click', () => {
    loadStats();
    loadDownloads();
});

// Cleanup WebSocket connections on page unload
window.addEventListener('beforeunload', () => {
    activeDownloads.forEach((ws) => ws.close());
    activeDownloads.clear();
});

// Initial load
loadStats();
loadDownloads();

// Auto-refresh every 5 seconds
setInterval(() => {
    loadStats();
    loadDownloads();
}, 5000);
