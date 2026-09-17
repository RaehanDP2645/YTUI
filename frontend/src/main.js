import './style.css';
import { CancelDownload, DownloadBatch, DownloadDefault, GetFileExistsPolicy, ResolveFileExists, SelectBatchFile, SetFileExistsPolicy } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <main class="shell">
    <section class="panel">
      <div class="header">
        <div>
          <h1>YTUI</h1>
          <p>YouTube video & music downloader</p>
        </div>
        <span class="badge">Default Mode</span>
      </div>

      <div class="workspace">
        <section class="controls-column">
          <div class="field">
            <label for="url">YouTube URL</label>
            <input id="url" type="url" placeholder="https://www.youtube.com/watch?v=..." />
          </div>

          <div class="batch-row">
            <button id="selectBatchBtn" class="secondary-button">Select .txt Batch File</button>
            
            <div class="batch-file">
              <span id="batchFileName">No batch file selected</span>
              <button
                id="clearBatchBtn"
                class="clear-batch-button"
                title="Unselected batch file"
                disabled
              >
                &times;
              </button>
            </div>
          </div>

          <div class="form-grid">
            <div class="field">
              <label for="type">Type</label>
              <select id="type">
                <option value="video">Video</option>
                <option value="music">Music</option>
              </select>
            </div>

            <div class="field">
              <label for="mode">Mode</label>
              <select id="mode">
                <option value="default">Default</option>
                <option value="custom">Custom</option>
              </select>
            </div>

            <div class="field">
              <label for="format">Format</label>
              <select id="format">
                <option value="mp4">MP4</option>
                <option value="mkv">MKV</option>
                <option value="webm">WEBM</option>
              </select>
            </div>

            <div class="field">
              <label for="quality">Quality</label>
              <select id="quality">
                <option value="1080">1080p (Best)</option>
                <option value="720">720p (Good)</option>
                <option value="480">480p (Not Bad)</option>
                <option value="360">360p (Small)</option>
              </select>
            </div>

            <div class="field">
              <label for="parallel">Parallel Downloads</label>
              <select id="parallel">
                <option value="1">1</option>
                <option value="2" selected>2</option>
                <option value="3">3</option>
                <option value="4">4</option>
              </select>
            </div>
          </div>

          <details class="advanced">
            <summary>Advanced Options</summary>

            <div class="advanced-grid">
              <div class="field">
                <label for="outputDir">Output Folder</label>
                <input id="outputDir" type="text" placeholder="Kosongkan untuk Downloads/YTUI" />
              </div>

              <div class="field">
                <label for="speedMode">Speed Mode</label>
                <select id="speedMode">
                  <option value="normal">Normal</option>
                  <option value="fast">Fast</option>
                  <option value="very-fast">Very Fast</option>
                </select>
              </div>
            </div>
          </details>

          <div class="action-row">
            <button id="downloadBtn">Download</button>
            <button id="batchDownloadBtn" class="secondary-button">Download Batch</button>
            <button id="cancelDownloadBtn" class="danger-button" disabled>Cancel Download</button>
          </div>

          <div class="exists-row">
            <label for="fileExistsPolicy">Jika file sudah ada</label>
            <select id="fileExistsPolicy">
              <option value="ask">Ask</option>
              <option value="rename">Rename</option>
              <option value="overwrite">Overwrite</option>
              <option value="abort">Abort</option>
            </select>
          </div>
      
        </section>
      
        <section class="progress-column">
          <section class="progress-card">
            <div class="progress-top">
              <strong id="progressStatus">Ready</strong>
              <span id="progressPercent">0%</span>
            </div>

            <div class="progress-track">
              <div id="progressFill" class="progress-fill"></div>
            </div>

            <div class="progress-meta">
              <span id="progressSpeed">Speed: -</span>
              <span id="progressEta">ETA: -</span>
            </div>

            <div id="status" class="status idle">Ready</div>
            <div id="downloadList" class="download-list"></div>
          </section>
        </section>
      </div>
    </section>
  </main>

  <div id="existsDialog" class="exists-dialog" hidden role="dialog" aria-modal="true" aria-labelledby="existsDialogTitle">
    <div class="exists-backdrop"></div>
    <div class="exists-box">
      <h2 id="existsDialogTitle">File already exists</h2>
      <p class="exists-desc">File yang akan didownload sudah ada:</p>
      <code id="existsPath"></code>

      <label class="exists-remember">
        <input type="checkbox" id="existsRemember" />
        <span>Remember my choice for all downloads</span>
      </label>

      <div class="exists-actions">
        <button id="existsRename" class="exists-btn-primary">Rename</button>
        <button id="existsOverwrite" class="exists-btn-secondary">Overwrite</button>
        <button id="existsAbort" class="exists-btn-danger">Abort</button>
      </div>
    </div>
  </div>
`;

const urlInput = document.querySelector('#url');
const typeSelect = document.querySelector('#type');
const modeSelect = document.querySelector('#mode');
const formatSelect = document.querySelector('#format');
const qualitySelect = document.querySelector('#quality');
const outputDirInput = document.querySelector('#outputDir');
const downloadBtn = document.querySelector('#downloadBtn');
const statusBox = document.querySelector('#status');
const progressStatus = document.querySelector('#progressStatus');
const progressPercent = document.querySelector('#progressPercent');
const progressFill = document.querySelector('#progressFill');
const progressSpeed = document.querySelector('#progressSpeed');
const progressEta = document.querySelector('#progressEta');
const downloadList = document.querySelector('#downloadList');
const downloadItems = new Map();
const downloadItemEls = new Map();
const selectBatchBtn = document.querySelector('#selectBatchBtn');
const batchFileName = document.querySelector('#batchFileName');
const clearBatchBtn = document.querySelector('#clearBatchBtn');
const batchDownloadBtn = document.querySelector('#batchDownloadBtn');
const parallelSelect = document.querySelector('#parallel');
const cancelDownloadBtn =document.querySelector('#cancelDownloadBtn');
const fileExistsPolicySelect = document.querySelector('#fileExistsPolicy');
const existsDialog = document.querySelector('#existsDialog');
const existsPath = document.querySelector('#existsPath');
const existsRemember = document.querySelector('#existsRemember');
const existsRenameBtn = document.querySelector('#existsRename');
const existsOverwriteBtn = document.querySelector('#existsOverwrite');
const existsAbortBtn = document.querySelector('#existsAbort');

let selectedBatchFile = '';

const existsQueue = [];
let existsActive = null;

EventsOn('download:progress', (event) => {
  updateProgress(event);
  updateDownloadList(event);
});

EventsOn('file-exists:ask', (data) => {
  existsQueue.push(data);
  showNextExistsDialog();
});

GetFileExistsPolicy().then((policy) => {
  if (policy) {
    fileExistsPolicySelect.value = policy;
  }
});

fileExistsPolicySelect.addEventListener('change', () => {
  SetFileExistsPolicy(fileExistsPolicySelect.value);
});

typeSelect.addEventListener('change', syncFormatOptions);
modeSelect.addEventListener('change', syncModeState);

downloadBtn.addEventListener('click', async () => {
  const url = urlInput.value.trim();

  if (!url) {
    setStatus('URL tidak boleh kosong.', 'error');
    return;
  }

  if (modeSelect.value === 'custom') {
    setStatus('Custom mode belum aktif. Kita aktifkan di tahap berikutnya.', 'error');
    return;
  }

  downloadItems.clear();
  downloadItemEls.clear();
  renderDownloadList();

  downloadBtn.disabled = true;
  cancelDownloadBtn.disabled = false;
  updateProgress({
    status: 'queued',
    percent: 0,
    speed: '-',
    eta: '-',
    message: 'Queued',
  });

  try {
    const result = await DownloadDefault({
      url,
      type: typeSelect.value,
      quality: qualitySelect.value,
      outputDir: outputDirInput.value.trim(),
    });

    setStatus(`${result.message}. Folder: ${result.outputDir}`, 'success');
  } catch (error) {
    const message = formatError(error).toLowerCase();

    if (message.includes('dibatalkan') || message.includes('canceled')) {
      setStatus('Download dibatalkan.', 'error');
    } else {
      setStatus('Download gagal', 'error');
    }
  } finally {
    downloadBtn.disabled = false;
    cancelDownloadBtn.disabled = true;
  }
});

cancelDownloadBtn.addEventListener('click', async () => {
  cancelDownloadBtn.disabled = true;
  setStatus('Membatalkan download...', 'loading');

  try {
    await CancelDownload();
  } catch (error) {
    setStatus(formatError(error), 'error');
  }
});

selectBatchBtn.addEventListener('click', async () => {
  try {
    const filePath = await SelectBatchFile();

    console.log('Selected batch file:', filePath);

    if (!filePath) {
      setStatus('Tidak ada file yang dipilih.', 'error');
      return;
    }

    selectedBatchFile = filePath;
    batchFileName.textContent = filePath;
    clearBatchBtn.disabled = false;
    setStatus(`File batch dipilih: ${filePath}`, 'success');
  } catch (error) {
    console.error(error);
    setStatus(formatError(error), 'error');
  }
});

clearBatchBtn.addEventListener('click', () => {
  selectedBatchFile = '';
  batchFileName.textContent = 'No batch file selected';
  clearBatchBtn.disabled = true;
  setStatus('File batch dibatalkan.', 'idle');
});

batchDownloadBtn.addEventListener('click', async () => {
  if (!selectedBatchFile) {
    setStatus('Pilih file .txt dulu!!', 'error');
    return;
  }

  downloadItems.clear();
  downloadItemEls.clear();
  renderDownloadList();

  batchDownloadBtn.disabled = true;
  downloadBtn.disabled = true;

  try {
    const result = await DownloadBatch({
      filePath: selectedBatchFile,
      type: typeSelect.value,
      quality: qualitySelect.value,
      outputDir: outputDirInput.value.trim(),
      parallel: Number(parallelSelect.value),
      skipErrors: true,
    });

    setStatus(
      `${result.message}. Total: ${result.total}, selesai: ${result.completed}, gagal: ${result.failed}`,
      'success'
    );
  } catch (error) {
    setStatus(formatError(error), 'error');
  } finally {
    batchDownloadBtn.disabled = false;
    downloadBtn.disabled = false;
  }
});

function syncFormatOptions() {
  if (typeSelect.value === 'music') {
    formatSelect.innerHTML = `
      <option value="mp3">MP3</option>
      <option value="m4a">M4A</option>
      <option value="opus">OPUS</option>
      <option value="wav">WAV</option>
      <option value="flac">FLAC</option>
    `;

    qualitySelect.innerHTML = `
      <option value="320">320k (Best)</option>
      <option value="256">256k (Good)</option>
      <option value="192">192k (Not Bad)</option>
      <option value="128">128k (Small)</option>
    `;

    return;
  }

  formatSelect.innerHTML = `
    <option value="mp4">MP4</option>
    <option value="mkv">MKV</option>
    <option value="webm">WEBM</option>
  `;

  qualitySelect.innerHTML = `
    <option value="1080">1080p (Best)</option>
    <option value="720">720p (Good)</option>
    <option value="480">480p (Not Bad)</option>
    <option value="360">360p (Small)</option>
  `;
}

function syncModeState() {
  const isCustom = modeSelect.value === 'custom';

  formatSelect.disabled = !isCustom;
  qualitySelect.disabled = false;
}


// Smooth progress display.
// Bar/percent dianimasikan mendekati nilai aktual terbaru dari yt-dlp dengan
// requestAnimationFrame: tidak pernah melebihi nilai aktual, tidak pernah
// mundur, dan konvergen cepat sehingga tetap akurat untuk download cepat/lambat.
const progressEaseRate = 8;
const progressEaseEpsilon = 0.05;

const progressViews = new Map(); // key -> view { target, current, nodes, pending, lastTerminal, ts }
let progressRaf = 0;

function progressView(key) {
  let view = progressViews.get(key);

  if (!view) {
    view = {
      target: 0,
      current: -1,
      nodes: null,
      pending: false,
      lastTerminal: false,
      ts: 0,
    };
    progressViews.set(key, view);
  }

  return view;
}

function applyProgressNode(view) {
  if (!view.nodes) {
    return;
  }

  const shown = Math.max(0, Math.min(100, view.current));
  view.nodes.fill.style.width = `${shown}%`;
  view.nodes.text.textContent = `${shown.toFixed(1)}%`;
}

function updateProgressView(key, nodes, event) {
  const view = progressView(key);

  if (nodes) {
    view.nodes = nodes;
  }

  const percent = Math.max(0, Math.min(100, Number(event.percent || 0)));
  const status = event.status;

  if (status === 'queued') {
    view.target = 0;
    view.current = 0;
    view.lastTerminal = false;
    view.pending = true;
  } else if (status === 'completed') {
    view.target = 100;
    view.current = 100;
    view.lastTerminal = true;
    view.pending = true;
  } else if (status === 'failed' || status === 'canceled') {
    view.target = Math.max(view.target, percent);
    view.lastTerminal = true;
    view.pending = true;
  } else {
    if (view.lastTerminal && percent === 0) {
      view.target = 0;
      view.current = 0;
      view.lastTerminal = false;
      view.pending = true;
    } else if (percent > view.target) {
      view.target = percent;
      view.pending = true;
    }
  }

  if (view.current < 0) {
    view.current = view.target;
  }

  startProgressLoop();
}

function startProgressLoop() {
  if (progressRaf) {
    return;
  }

  progressRaf = requestAnimationFrame(function tick(ts) {
    progressRaf = 0;
    let keepRunning = false;

    for (const view of progressViews.values()) {
      if (!view.nodes) {
        continue;
      }

      const gap = view.target - view.current;
      const moving = Math.abs(gap) > progressEaseEpsilon;

      if (view.pending && !moving) {
        view.current = view.target;
        applyProgressNode(view);
      } else if (moving) {
        const dt = view.ts ? Math.min((ts - view.ts) / 1000, 0.25) : 0.016;
        const k = 1 - Math.exp(-progressEaseRate * dt);
        view.current += gap * k;

        if (Math.abs(view.target - view.current) <= progressEaseEpsilon) {
          view.current = view.target;
        }

        applyProgressNode(view);
        keepRunning = true;
      }

      view.pending = false;
      view.ts = ts;
    }

    if (keepRunning) {
      progressRaf = requestAnimationFrame(tick);
    }
  });
}

function clearProgressViews() {
  for (const view of progressViews.values()) {
    view.nodes = null;
    view.pending = false;
  }
}

function updateProgress(event) {
  progressStatus.textContent = event.message || event.status || 'Working';
  progressSpeed.textContent = `Speed: ${event.speed || '-'}`;
  progressEta.textContent = `ETA: ${event.eta || '-'}`;

  updateProgressView('main', { fill: progressFill, text: progressPercent }, event);

  if (event.status === 'completed') {
    setStatus('Download selesai.', 'success');
  } else if (event.status === 'failed') {
    setStatus('Download gagal.', 'error');
  } else if (event.status === 'canceled') {
    setStatus('Download dibatalkan.', 'error');
  } else if (event.status === 'downloading') {
    setStatus('Downloading...', 'loading');
  }
}

function updateDownloadList(event) {
  if (!event.url) {
    return;
  }

  const current = downloadItems.get(event.url) || {
    url: event.url,
    status: 'queued',
    percent: 0,
    speed: '-',
    eta: '-',
    message: 'Queued',
  };

  const next = {
    ...current,
    status: event.status || current.status,
    percent: Number(event.percent ?? current.percent ?? 0),
    speed: event.speed || current.speed || '-',
    eta: event.eta || current.eta || '-',
    message: event.message || current.message || event.status || 'Working',
  };

  downloadItems.set(event.url, next);
  updateDownloadItemDom(event.url, next);
}

function updateDownloadItemDom(url, item) {
  let els = downloadItemEls.get(url);

  if (!els) {
    const root = document.createElement('article');
    root.className = 'download-item';
    root.innerHTML = `
      <div class="download-item-top">
        <strong></strong>
        <span class="download-status"></span>
      </div>
      <div class="download-item-progress"><div></div></div>
      <div class="download-item-meta">
        <span class="pct"></span>
        <span class="spd"></span>
        <span class="eta"></span>
      </div>
    `;

    els = {
      root,
      title: root.querySelector(':scope > .download-item-top > strong'),
      status: root.querySelector(':scope > .download-item-top > .download-status'),
      fill: root.querySelector(':scope > .download-item-progress > div'),
      percent: root.querySelector(':scope > .download-item-meta > .pct'),
      speed: root.querySelector(':scope > .download-item-meta > .spd'),
      eta: root.querySelector(':scope > .download-item-meta > .eta'),
    };

    downloadList.appendChild(root);
    downloadItemEls.set(url, els);
  }

  const percent = Math.max(0, Math.min(100, Number(item.percent || 0)));

  els.title.textContent = item.url;
  els.title.title = item.url;
  els.status.textContent = item.status;
  els.status.className = `download-status ${item.status}`;
  els.speed.textContent = `Speed: ${item.speed || '-'}`;
  els.eta.textContent = `ETA: ${item.eta || '-'}`;

  updateProgressView(
    url,
    { fill: els.fill, text: els.percent },
    { status: item.status, percent }
  );
}

function showNextExistsDialog() {
  if (existsActive || existsQueue.length === 0) {
    return;
  }

  existsActive = existsQueue.shift();
  existsPath.textContent = existsActive.path;
  existsPath.title = existsActive.path;
  existsDialog.hidden = false;
}

function finishExistsDialog(choice) {
  if (!existsActive) {
    return;
  }

  const id = existsActive.id;
  const remember = existsRemember.checked;
  existsActive = null;

  existsDialog.hidden = true;
  existsRemember.checked = false;

  if (remember) {
    fileExistsPolicySelect.value = choice;
  }

  ResolveFileExists(id, choice, remember);
  showNextExistsDialog();
}

existsAbortBtn.addEventListener('click', () => finishExistsDialog('abort'));
existsOverwriteBtn.addEventListener('click', () => finishExistsDialog('overwrite'));
existsRenameBtn.addEventListener('click', () => finishExistsDialog('rename'));

function renderDownloadList() {
  downloadList.innerHTML = '';
  downloadItemEls.clear();
  clearProgressViews();

  if (downloadItems.size === 0) {
    return;
  }

  for (const [url, item] of downloadItems) {
    updateDownloadItemDom(url, item);
  }
}

function setStatus(message, type) {
  statusBox.textContent = message;
  statusBox.className = `status ${type}`;
}

function formatError(error) {
  if (typeof error === 'string') {
    return error;
  }

  if (error?.message) {
    return error.message;
  }

  return 'Terjadi error saat download.';
}

syncFormatOptions();
syncModeState();
