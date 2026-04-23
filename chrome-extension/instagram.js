/**
 * X-Extract Downloader — content script for www.instagram.com
 *
 * Injects a download button into every post's action bar (feed, profile grid,
 * reels, and individual post/reel pages).  Clicking it calls
 * POST http://localhost:9091/api/v1/downloads with the canonical post URL.
 */

const API_URL = 'http://localhost:9091/api/v1/downloads';
const MARKER  = 'xei-btn';   // class used to detect already-injected buttons

// ─── SVG icons ─────────────────────────────────────────────────────────────────

const ICON_DOWNLOAD = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M12 16l-5-5h3V4h4v7h3l-5 5zm-7 2h14v2H5v-2z"/>
  </svg>`;

const ICON_SPINNER = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" class="xei-spin">
    <path d="M12 2a10 10 0 1 0 10 10A10 10 0 0 0 12 2zm0 18a8 8 0 1 1 8-8 8 8 0 0 1-8 8z" opacity=".3"/>
    <path d="M12 2a10 10 0 0 1 10 10h-2a8 8 0 0 0-8-8z"/>
  </svg>`;

const ICON_OK = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/>
  </svg>`;

const ICON_ERR = `
  <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
  </svg>`;

// ─── Styles ─────────────────────────────────────────────────────────────────────

function injectStyles() {
  if (document.getElementById('xei-styles')) return;
  const style = document.createElement('style');
  style.id = 'xei-styles';
  style.textContent = `
    .xei-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: 0 0 auto;
      background: none;
      border: none;
      cursor: pointer;
      padding: 8px;
      width: 40px;
      height: 40px;
      box-sizing: border-box;
      margin: 0;
      color: inherit;
      border-radius: 9999px;
      transition: opacity .15s;
      position: relative;
    }
    .xei-btn:hover { opacity: 0.6; }
    .xei-btn.xei-ok  { color: #00ba7c !important; }
    .xei-btn.xei-err { color: #ed4956 !important; }
    .xei-btn .xei-tooltip {
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
      z-index: 99999;
    }
    .xei-btn:hover .xei-tooltip { display: block; }
    @keyframes xei-spin-anim { to { transform: rotate(360deg); } }
    .xei-spin { animation: xei-spin-anim .7s linear infinite; }

    /*
     * Prevent the action-bar from wrapping once our button is injected.
     * We target every descendant of the <section> so we hit the actual
     * flex container regardless of how deeply nested it is.
     * :has() is Chrome 105+ — always fine in a Chrome extension.
     * This <style> tag persists through React re-renders, unlike inline styles.
     */
    article:has(.xei-btn) section,
    article:has(.xei-btn) section > div,
    article:has(.xei-btn) section > div > div {
      flex-wrap: nowrap !important;
    }
  `;
  document.head.appendChild(style);
}

// ─── URL extraction ─────────────────────────────────────────────────────────────

/**
 * Extract the canonical Instagram post/reel URL from an article element.
 *
 * Four strategies (tried in order):
 *   1. window.location — already on a /p/ or /reel/ page
 *   2. <time> → parent <a href="/p/…"> (most reliable in feed)
 *   3. Any <a href="/p/…"> or <a href="/reel/…"> inside the article
 *   4. window.location.pathname on a modal overlay (/p/ still shows in URL)
 */
function getPostUrl(article) {
  const origin = 'https://www.instagram.com';

  // 1. Already on a dedicated post/reel page
  const path = window.location.pathname;
  if (/^\/(p|reel)\//.test(path)) {
    return origin + path.replace(/\/$/, '') + '/';
  }

  // 2. <time> permalink (feed, explore grid detail)
  const timeEl = article.querySelector('time');
  if (timeEl) {
    const link = timeEl.closest('a[href]');
    if (link) {
      const href = link.getAttribute('href');
      if (href && /\/(p|reel)\//.test(href)) {
        return origin + href.split('?')[0];
      }
    }
  }

  // 3. Any <a> whose href is exactly a post or reel path
  for (const a of article.querySelectorAll('a[href]')) {
    const href = a.getAttribute('href');
    if (href && /^\/(p|reel)\/[\w-]+\/?$/.test(href)) {
      return origin + href.split('?')[0];
    }
  }

  // 4. URL changed to post on modal open (profile grid click)
  if (/^\/(p|reel)\//.test(window.location.pathname)) {
    return origin + window.location.pathname;
  }

  return null;
}

// ─── Action bar detection ───────────────────────────────────────────────────────

/**
 * Find the flex-row element that IS the action bar, and the wrapper element
 * inside it that contains the Save/Bookmark button.
 *
 * Instagram's structure (simplified):
 *
 *   <section>
 *     <div>                          ← flexRow  (what we want to insert into)
 *       <span><button Like/></span>
 *       <span>...</span>             ← comment, share wrappers
 *       <div style="margin-left:auto"> ← saveWrapper  (insert our btn before this)
 *         <button aria-label="Save"/>
 *       </div>
 *     </div>
 *   </section>
 *
 * We anchor on the Like button, walk UP until we reach an ancestor that ALSO
 * contains the Save button — that ancestor is the flex row.  Then we find
 * Save's direct-child-of-flexRow wrapper to use as the insertion sibling.
 *
 * Returns { flexRow, saveWrapper } or null.
 */
function findActionBar(article) {
  const LIKE_SEL = '[aria-label="Like"], [aria-label="Unlike"], [aria-label="like"], [aria-label="unlike"]';
  const SAVE_SEL = '[aria-label="Save"], [aria-label="Remove"], [aria-label="save"], [aria-label="remove"]';

  const likeEl = article.querySelector(LIKE_SEL);
  if (!likeEl) return null;

  // Walk up from the Like button until the element also contains the Save button
  let flexRow = likeEl.parentElement;
  while (flexRow && flexRow !== article) {
    if (flexRow.querySelector(SAVE_SEL)) break;
    flexRow = flexRow.parentElement;
  }
  if (!flexRow || flexRow === article) return null;

  // Find Save's direct-child-of-flexRow wrapper (the element we insert before)
  const saveEl = flexRow.querySelector(SAVE_SEL);
  let saveWrapper = saveEl;
  while (saveWrapper && saveWrapper.parentElement !== flexRow) {
    saveWrapper = saveWrapper.parentElement;
  }

  return { flexRow, saveWrapper: saveWrapper || null };
}

// ─── API call ──────────────────────────────────────────────────────────────────

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

// ─── Button ────────────────────────────────────────────────────────────────────

function createButton(postUrl) {
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.className = MARKER;
  btn.setAttribute('aria-label', 'Download with X-Extract');
  btn.dataset.xeiUrl = postUrl;

  const tooltip = document.createElement('span');
  tooltip.className = 'xei-tooltip';
  tooltip.textContent = 'Download';

  btn.innerHTML = ICON_DOWNLOAD;
  btn.appendChild(tooltip);

  btn.addEventListener('click', async (e) => {
    e.preventDefault();
    e.stopPropagation();

    btn.innerHTML = ICON_SPINNER;
    btn.appendChild(tooltip);
    btn.disabled = true;
    tooltip.textContent = 'Adding…';

    try {
      await addDownload(postUrl);

      btn.innerHTML = ICON_OK;
      btn.appendChild(tooltip);
      btn.classList.add('xei-ok');
      tooltip.textContent = 'Added!';

      setTimeout(() => {
        btn.innerHTML = ICON_DOWNLOAD;
        btn.appendChild(tooltip);
        btn.classList.remove('xei-ok');
        btn.disabled = false;
        tooltip.textContent = 'Download';
      }, 2500);
    } catch (err) {
      btn.innerHTML = ICON_ERR;
      btn.appendChild(tooltip);
      btn.classList.add('xei-err');

      const msg = err.message.includes('Failed to fetch')
        ? 'Server offline'
        : err.message;
      tooltip.textContent = msg;
      tooltip.style.display = 'block';

      setTimeout(() => {
        btn.innerHTML = ICON_DOWNLOAD;
        btn.appendChild(tooltip);
        btn.classList.remove('xei-err');
        btn.disabled = false;
        tooltip.textContent = 'Download';
        tooltip.style.display = '';
      }, 3000);
    }
  });

  return btn;
}

// ─── Injection ─────────────────────────────────────────────────────────────────

function injectButton(article) {
  // Skip if already injected
  if (article.querySelector(`.${MARKER}`)) return;

  const url = getPostUrl(article);
  if (!url) return;

  const bar = findActionBar(article);
  if (!bar) return;

  const { flexRow, saveWrapper } = bar;
  const btn = createButton(url);

  // Insert inside the flex row, just before the Save/Bookmark wrapper
  // so our button sits between Share and Save — never outside the row.
  if (saveWrapper) {
    flexRow.insertBefore(btn, saveWrapper);
  } else {
    flexRow.appendChild(btn);
  }

  console.debug('[xei] injected download button →', url);
}

function injectAll() {
  document.querySelectorAll('article').forEach(injectButton);
}

// ─── Bootstrap ─────────────────────────────────────────────────────────────────

injectStyles();
injectAll();

// Instagram is a SPA — debounce the observer so we don't hammer injectAll
// on every tiny DOM mutation (images loading, etc.)
let debounceTimer = null;
const observer = new MutationObserver(() => {
  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(injectAll, 300);
});
observer.observe(document.body, { childList: true, subtree: true });
