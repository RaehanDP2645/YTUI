import './style.css';
import { DownloadDefault } from '../wailsjs/go/main/App';
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

      <div class="field">
        <label for="url">YouTube URL</label>
        <input id="url" type="url" placeholder="https://www.youtube.com/watch?v=..." />
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
            <option value="best">Best</option>
            <option value="1080">1080p</option>
            <option value="720">720p</option>
            <option value="480">480p</option>
            <option value="360">360p</option>
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

      <button id="downloadBtn">Download</button>

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
      </section>
    </section>
  </main>
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

EventsOn('download:progress', (event) => {
  updateProgress(event);
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

  downloadBtn.disabled = true;
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
    setStatus(formatError(error), 'error');
  } finally {
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
      <option value="best">Best</option>
      <option value="320">320k</option>
      <option value="256">256k</option>
      <option value="192">192k</option>
      <option value="128">128k</option>
    `;

    return;
  }

  formatSelect.innerHTML = `
    <option value="mp4">MP4</option>
    <option value="mkv">MKV</option>
    <option value="webm">WEBM</option>
  `;

  qualitySelect.innerHTML = `
    <option value="best">Best</option>
    <option value="1080">1080p</option>
    <option value="720">720p</option>
    <option value="480">480p</option>
    <option value="360">360p</option>
  `;
}

function syncModeState() {
  const isCustom = modeSelect.value === 'custom';

  formatSelect.disabled = !isCustom;
  qualitySelect.disabled = false;
}


function updateProgress(event) {
  const percent = Number(event.percent || 0);
  const safePercent = Math.max(0, Math.min(100, percent));

  progressStatus.textContent = event.message || event.status || 'Working';
  progressPercent.textContent = `${safePercent.toFixed(1)}%`;
  progressFill.style.width = `${safePercent}%`;
  progressSpeed.textContent = `Speed: ${event.speed || '-'}`;
  progressEta.textContent = `ETA: ${event.eta || '-'}`;

  if (event.status === 'completed') {
    setStatus('Download selesai.', 'success');
  } else if (event.status === 'failed') {
    setStatus('Download gagal.', 'error');
  } else if (event.status === 'downloading') {
    setStatus('Downloading...', 'loading');
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
