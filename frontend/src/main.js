import './style.css';
import { DownloadDefault } from '../wailsjs/go/main/App';

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

      <div id="status" class="status idle">Ready</div>
    </section>
  </main>
`;

const urlInput = document.querySelector('#url');
const typeSelect = document.querySelector('#type');
const outputDirInput = document.querySelector('#outputDir');
const downloadBtn = document.querySelector('#downloadBtn');
const statusBox = document.querySelector('#status');

downloadBtn.addEventListener('click', async () => {
  const url = urlInput.value.trim();

  if (!url) {
    setStatus('URL tidak boleh kosong.', 'error');
    return;
  }

  downloadBtn.disabled = true;
  setStatus('Downloading...', 'loading');

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
