const state = { mode: 'generate', lastImage: null };

async function api(path, options = {}) {
  const headers = options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' };
  const response = await fetch(path, { credentials: 'include', headers, ...options });
  const data = await response.json().catch(() => ({}));
  const isLoginPage = window.location.pathname.includes('login') || window.location.pathname.includes('register');
  if (response.status === 401 && !isLoginPage) {
    window.location.href = '/login.html';
  }
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}

function setStatus(message, type = '') {
  const line = document.getElementById('statusLine');
  line.textContent = message || '';
  line.className = `status-line ${type}`;
}

async function init() {
  const [profile, config] = await Promise.all([api('/api/profile'), api('/api/config')]);
  document.getElementById('currentUser').textContent = profile.user.username;
  document.getElementById('siteName').textContent = config.site_name || 'AI 图片工作台';
  document.getElementById('siteIcon').textContent = config.site_icon || 'AI';
  document.getElementById('adminBadge').classList.toggle('hidden', !profile.user.is_admin);
  document.querySelectorAll('.admin-link').forEach((item) => {
    item.classList.toggle('hidden', !profile.user.is_admin);
  });
  fillSelect('modelSelect', config.models || ['gpt-image-1']);
  fillSelect('sizeSelect', config.sizes || ['1024x1024']);
  bindRatioSync();
  bindModes();
  bindForm();
  bindLogout();
  bindShare();
}

function fillSelect(id, values) {
  const select = document.getElementById(id);
  select.innerHTML = values.map((value) => `<option value="${escapeHTML(value)}">${escapeHTML(value)}</option>`).join('');
}

function bindRatioSync() {
  const ratioSelect = document.getElementById('ratioSelect');
  const sizeSelect = document.getElementById('sizeSelect');
  const ratios = Array.from(ratioSelect.options).map((option) => option.value);

  function parsePair(value) {
    const match = String(value).match(/^(\d+)\s*[:x×]\s*(\d+)$/i);
    if (!match) return null;
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!width || !height) return null;
    return { width, height, value: width / height };
  }

  function closestRatioForSize(size) {
    const parsedSize = parsePair(size);
    if (!parsedSize) return '';
    let closest = '';
    let closestDistance = Number.POSITIVE_INFINITY;
    ratios.forEach((ratio) => {
      const parsedRatio = parsePair(ratio);
      if (!parsedRatio) return;
      const distance = Math.abs(parsedSize.value - parsedRatio.value) / parsedRatio.value;
      if (distance < closestDistance) {
        closest = ratio;
        closestDistance = distance;
      }
    });
    return closestDistance <= 0.04 ? closest : '';
  }

  function closestSizeForRatio(ratio) {
    const parsedRatio = parsePair(ratio);
    if (!parsedRatio) return '';
    let closest = '';
    let closestDistance = Number.POSITIVE_INFINITY;
    Array.from(sizeSelect.options).forEach((option) => {
      const parsedSize = parsePair(option.value);
      if (!parsedSize) return;
      const distance = Math.abs(parsedSize.value - parsedRatio.value) / parsedRatio.value;
      if (distance < closestDistance) {
        closest = option.value;
        closestDistance = distance;
      }
    });
    return closestDistance <= 0.04 ? closest : '';
  }

  function syncSizeFromRatio() {
    const size = closestSizeForRatio(ratioSelect.value);
    if (!size) return;
    sizeSelect.value = size;
  }

  function syncRatioFromSize() {
    const ratio = closestRatioForSize(sizeSelect.value);
    if (ratio) {
      ratioSelect.value = ratio;
    }
  }

  ratioSelect.addEventListener('change', () => {
    syncSizeFromRatio();
  });
  sizeSelect.addEventListener('change', () => {
    syncRatioFromSize();
  });
  syncRatioFromSize();
}

function bindModes() {
  document.querySelectorAll('.mode-tab').forEach((button) => {
    button.addEventListener('click', () => {
      state.mode = button.dataset.mode;
      document.querySelectorAll('.mode-tab').forEach((item) => {
        item.classList.remove('active');
      });
      button.classList.add('active');
      document.getElementById('uploadPanel').classList.toggle('hidden', state.mode === 'generate');
      document.getElementById('promptInput').placeholder = state.mode === 'remove-bg'
        ? '可留空，默认使用 remove background，也可以写下保留主体的要求...'
        : '描述你想生成的画面、风格、光线、构图...';
      document.getElementById('generateBtn').textContent = state.mode === 'remove-bg' ? '去除背景' : '生成图片';
    });
  });
}

function bindForm() {
  const form = document.getElementById('imageForm');
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = document.getElementById('generateBtn');
    button.disabled = true;
    setStatus('正在生成图片，这通常需要几秒钟...');
    try {
      const result = state.mode === 'generate' ? await submitGenerate() : await submitUpload();
      state.lastImage = result;
      renderResult(result);
      setStatus('生成完成，已保存到画廊。', 'success');
    } catch (error) {
      setStatus(error.message, 'error');
    } finally {
      button.disabled = false;
    }
  });
}

async function submitGenerate() {
  const payload = {
    prompt: document.getElementById('promptInput').value.trim(),
    model: document.getElementById('modelSelect').value,
    ratio: document.getElementById('ratioSelect').value,
    size: document.getElementById('sizeSelect').value,
  };
  return api('/api/images/generate', { method: 'POST', body: JSON.stringify(payload) });
}

async function submitUpload() {
  const fileInput = document.getElementById('sourceImage');
  if (!fileInput.files[0]) throw new Error('请先上传源图片');
  const form = new FormData();
  form.append('image', fileInput.files[0]);
  form.append('prompt', document.getElementById('promptInput').value.trim());
  form.append('model', document.getElementById('modelSelect').value);
  form.append('ratio', document.getElementById('ratioSelect').value);
  form.append('size', document.getElementById('sizeSelect').value);
  const endpoint = state.mode === 'remove-bg' ? '/api/images/remove-bg' : '/api/images/edit';
  return api(endpoint, { method: 'POST', body: form });
}

function renderResult(image) {
  document.getElementById('emptyState').classList.add('hidden');
  const card = document.getElementById('resultCard');
  card.classList.remove('hidden');
  document.getElementById('resultImage').src = `${image.image}&v=${Date.now()}`;
  document.getElementById('resultPrompt').textContent = image.prompt || '无提示词';
}

function bindShare() {
  document.getElementById('shareResult').addEventListener('click', async () => {
    if (!state.lastImage) return;
    try {
      const result = await api('/api/share', { method: 'POST', body: JSON.stringify({ image_id: state.lastImage.id }) });
      const url = new URL(result.url, window.location.origin).toString();
      await navigator.clipboard.writeText(url).catch(() => {});
      setStatus(`分享链接已创建：${url}`, 'success');
    } catch (error) {
      setStatus(error.message, 'error');
    }
  });
}

function bindLogout() {
  document.getElementById('logoutBtn').addEventListener('click', async () => {
    await api('/api/logout', { method: 'POST', body: '{}' }).catch(() => {});
    window.location.href = '/login.html';
  });
}

function escapeHTML(value) {
  return String(value).replace(/[&<>'"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char]));
}

init().catch((error) => setStatus(error.message, 'error'));
