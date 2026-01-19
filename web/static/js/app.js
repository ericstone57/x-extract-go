const API_BASE = '/api/v1';

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
                </div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Failed to load downloads:', error);
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

// Initial load
loadStats();
loadDownloads();

// Auto-refresh every 5 seconds
setInterval(() => {
    loadStats();
    loadDownloads();
}, 5000);

