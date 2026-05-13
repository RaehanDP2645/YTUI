import './style.css';
import { DownloadDefault } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <main class="shell">
    <section class="panel">
      <div class="header">
        <h1>YTUI</h1>
        <p>YouTube video & music downloader</p>
      </div>

      <div class="field">
        <label for="url">YouTube URL</label>
        <input id="url" type="url" placeholder="https://www.youtube.com/watch?v=..." />
      </div>

      <div class="grid">
        <div class="field">
          <label for="type">Type</label>
          <select id="type">
            <option value="video">Video MP4</option>
            <option value="music">Music MP3</option>
          </select>
        </div>

        <div class="field">
          <label for="outputDir">Output Folder</label>
          <input id="outputDir" type="text" placeholder="Kosongkan untuk Downloads/YTUI" />
        </div>
      </div>

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

downloadBtn.addEventListener('click', async () => {
  const url = urlInput.value.trim();

  if (!url) {
    setStatus('URL tidak boleh kosong.', 'error');
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
      outputDir: outputDirInput.value.trim(),
    });

    setStatus(`${result.message}. Folder: ${result.outputDir}`, 'success');
  } catch (error) {
    setStatus(formatError(error), 'error');
  } finally {
    downloadBtn.disabled = false;
  }
});

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
