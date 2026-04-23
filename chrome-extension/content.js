/**
 * X-Extract Downloader — content script for x.com / twitter.com
 *
 * Injects a download button into every tweet's action bar.
 * Clicking it calls POST http://localhost:9091/api/v1/downloads with the tweet URL.
 */

const API_URL = 'http://localhost:9091/api/v1/downloads';

// ─── SVG icons ────────────────────────────────────────────────────────────────

const ICON_DOWNLOAD = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M12 16l-5-5h3V4h4v7h3l-5 5zm-7 2h14v2H5v-2z"/>
  </svg>`;

const ICON_SPINNER = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" class="xe-spin">
    <path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm0 18a8 8 0 1 1 8-8 8 8 0 0 1-8 8z"
          opacity=".3"/>
    <path d="M12 2a10 10 0 0 1 10 10h-2a8 8 0 0 0-8-8z"/>
  </svg>`;

const ICON_OK = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/>
  </svg>`;

const ICON_ERR = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1
             15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
  </svg>`;

// ─── Styles (injected once) ────────────────────────────────────────────────────

function injectStyles() {
  if (document.getElementById('xe-styles')) return;
  const style = document.createElement('style');
  style.id = 'xe-styles';
  style.textContent = `
    .xe-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      background: none;
      border: none;
      cursor: pointer;
      padding: 0;
      margin: 0;
      color: rgb(113, 118, 123);
      border-radius: 9999px;
      transition: color .15s, background .15s;
      position: relative;
      width: 34px;
      height: 34px;
    }
    .xe-btn:hover {
      color: rgb(29, 155, 240);
      background: rgba(29, 155, 240, .1);
    }
    .xe-btn.xe-ok {
      color: rgb(0, 186, 124);
    }
    .xe-btn.xe-err {
      color: rgb(249, 24, 128);
    }
    .xe-btn .xe-tooltip {
      display: none;
      position: absolute;
      bottom: calc(100% + 6px);
      left: 50%;
      transform: translateX(-50%);
      background: rgba(0,0,0,.85);
      color: #fff;
      font-size: 11px;
      white-space: nowrap;
      padding: 3px 7px;
      border-radius: 4px;
      pointer-events: none;
      z-index: 9999;
    }
    .xe-btn:hover .xe-tooltip {
      display: block;
    }
    @keyframes xe-spin-anim {
      to { transform: rotate(360deg); }
    }
    .xe-spin {
      animation: xe-spin-anim .7s linear infinite;
    }
  `;
  document.head.appendChild(style);
}

// ─── URL extraction ────────────────────────────────────────────────────────────

/**
 * Given a tweet <article>, return the canonical status URL
 * (https://x.com/username/status/TWEET_ID) or null.
 */
function getTweetUrl(article) {
  // The timestamp <time> element is wrapped in a permalink <a>
  const timeEl = article.querySelector('time');
  if (!timeEl) return null;
  const link = timeEl.closest('a');
  if (!link) return null;
  const href = link.getAttribute('href'); // e.g. "/username/status/12345"
  if (!href || !href.includes('/status/')) return null;
  return 'https://x.com' + href.split('?')[0]; // strip query params
}

// ─── API call ─────────────────────────────────────────────────────────────────

async function addDownload(url) {
  const resp = await fetch(API_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url }),
  });
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

// ─── Button ───────────────────────────────────────────────────────────────────

function createButton(tweetUrl) {
  const btn = document.createElement('button');
  btn.className = 'xe-btn';
  btn.setAttribute('aria-label', 'Download with X-Extract');
  btn.dataset.xeUrl = tweetUrl;

  const tooltip = document.createElement('span');
  tooltip.className = 'xe-tooltip';
  tooltip.textContent = 'Download';

  btn.innerHTML = ICON_DOWNLOAD;
  btn.appendChild(tooltip);

  btn.addEventListener('click', async (e) => {
    e.preventDefault();
    e.stopPropagation();

    // --- loading ---
    btn.innerHTML = ICON_SPINNER;
    btn.appendChild(tooltip);
    btn.disabled = true;
    tooltip.textContent = 'Adding…';

    try {
      await addDownload(tweetUrl);

      // --- success ---
      btn.innerHTML = ICON_OK;
      btn.appendChild(tooltip);
      btn.classList.add('xe-ok');
      tooltip.textContent = 'Added!';

      setTimeout(() => {
        btn.innerHTML = ICON_DOWNLOAD;
        btn.appendChild(tooltip);
        btn.classList.remove('xe-ok');
        btn.disabled = false;
        tooltip.textContent = 'Download';
      }, 2500);
    } catch (err) {
      // --- error ---
      btn.innerHTML = ICON_ERR;
      btn.appendChild(tooltip);
      btn.classList.add('xe-err');

      const msg = err.message.includes('Failed to fetch')
        ? 'Server offline'
        : err.message;
      tooltip.textContent = msg;

      // Keep tooltip visible on error without hover
      tooltip.style.display = 'block';

      setTimeout(() => {
        btn.innerHTML = ICON_DOWNLOAD;
        btn.appendChild(tooltip);
        btn.classList.remove('xe-err');
        btn.disabled = false;
        tooltip.textContent = 'Download';
        tooltip.style.display = '';
      }, 3000);
    }
  });

  return btn;
}

// ─── Injection ────────────────────────────────────────────────────────────────

/**
 * Injects the download button into a tweet article's action bar.
 * Safe to call multiple times — skips articles that already have a button.
 */
function injectButton(article) {
  if (article.querySelector('.xe-btn')) return; // already injected

  const url = getTweetUrl(article);
  if (!url) return;

  // X.com renders action buttons inside a <div role="group">
  // There are typically two: the main actions row and an accessibility group.
  // We want the one that contains the reply/retweet/like buttons.
  const groups = article.querySelectorAll('[role="group"]');
  const actionGroup = Array.from(groups).find(g =>
    g.querySelector('[data-testid="reply"], [data-testid="retweet"], [data-testid="like"]')
  );
  if (!actionGroup) return;

  const wrapper = document.createElement('div');
  wrapper.style.cssText = 'display:inline-flex;align-items:center;';
  wrapper.appendChild(createButton(url));
  actionGroup.appendChild(wrapper);
}

function injectAll() {
  document.querySelectorAll('article[data-testid="tweet"]').forEach(injectButton);
}

// ─── Bootstrap ────────────────────────────────────────────────────────────────

injectStyles();
injectAll();

// X.com is a SPA — new tweets load dynamically; watch the DOM for changes.
const observer = new MutationObserver(() => injectAll());
observer.observe(document.body, { childList: true, subtree: true });
