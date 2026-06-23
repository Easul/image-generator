let currentImage = null;
let currentGalleryItems = [];
let selectedIds = new Set();

async function api(path, options = {}) {
  const response = await fetch(path, { credentials: 'include', headers: { 'Content-Type': 'application/json' }, ...options });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) window.location.href = '/login.html';
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}

function setStatus(message, type = '') {
  const status = document.getElementById('galleryStatus');
  status.textContent = message || '';
  status.className = `status-line ${type}`;
}

async function initGallery() {
  const profile = await api('/api/profile');
  document.getElementById('currentUser').textContent = profile.user.username;
  document.getElementById('adminBadge').classList.toggle('hidden', !profile.user.is_admin);
  document.querySelectorAll('.admin-link').forEach((item) => {
    item.classList.toggle('hidden', !profile.user.is_admin);
  });
  bindLogout();
  bindModal();
  bindToolbar();
  bindGrid();
  document.getElementById('refreshGallery').addEventListener('click', loadGallery);
  await loadGallery();
}

async function loadGallery() {
  setStatus('正在加载画廊...');
  const data = await api('/api/gallery');
  const grid = document.getElementById('galleryGrid');
  currentGalleryItems = data.items || [];
  const validIds = new Set(currentGalleryItems.map((item) => String(item.id)));
  selectedIds = new Set(Array.from(selectedIds).filter((id) => validIds.has(id)));
  if (currentGalleryItems.length === 0) {
    grid.classList.add('is-empty');
    grid.innerHTML = '<div class="empty-state compact-empty-state gallery-empty-state"><h2>暂无图片</h2><p>回到工作台生成第一张作品。</p><a class="ghost-link compact-link" href="/">回到工作台</a></div>';
    setStatus('');
    selectedIds.clear();
    updateToolbar();
    return;
  }
  grid.classList.remove('is-empty');
  grid.innerHTML = currentGalleryItems.map(cardHTML).join('');
  updateToolbar();
  setStatus(`已加载 ${currentGalleryItems.length} 张图片`, 'success');
}

function cardHTML(image) {
  const isSelected = selectedIds.has(String(image.id));
  return `<article class="gallery-card" data-id="${image.id}">
    <div class="card-media">
      <img src="${image.image}" alt="${escapeHTML(image.prompt || 'AI image')}" loading="lazy">
      <div class="card-overlay">
        <input type="checkbox" class="card-checkbox" ${isSelected ? 'checked' : ''}>
        <button type="button" class="card-delete-btn" title="删除">×</button>
      </div>
    </div>
    <h3>${escapeHTML(image.prompt || '无提示词')}</h3>
    <p>${escapeHTML(image.model || '')} · ${escapeHTML(image.resolution || '')}</p>
  </article>`;
}

function updateToolbar() {
  const toolbar = document.getElementById('galleryToolbar');
  const countSpan = document.getElementById('selectedCount');
  const selectAllBtn = document.getElementById('selectAllBtn');
  const cancelSelectionBtn = document.getElementById('cancelSelectionBtn');
  const batchDeleteBtn = document.getElementById('batchDeleteBtn');
  const totalCards = document.querySelectorAll('.gallery-card').length;
  toolbar.classList.toggle('hidden', totalCards === 0);
  countSpan.textContent = selectedIds.size > 0 ? `已选择 ${selectedIds.size} 项` : `共 ${totalCards} 项，勾选后可批量删除`;
  selectAllBtn.disabled = totalCards === 0;
  cancelSelectionBtn.disabled = selectedIds.size === 0;
  batchDeleteBtn.disabled = selectedIds.size === 0;
}

function bindToolbar() {
  document.getElementById('selectAllBtn').addEventListener('click', () => {
    document.querySelectorAll('.card-checkbox').forEach((cb) => {
      cb.checked = true;
      selectedIds.add(cb.closest('.gallery-card').dataset.id);
    });
    updateToolbar();
  });
  document.getElementById('cancelSelectionBtn').addEventListener('click', () => {
    document.querySelectorAll('.card-checkbox').forEach((cb) => {
      cb.checked = false;
    });
    selectedIds.clear();
    updateToolbar();
  });
  document.getElementById('batchDeleteBtn').addEventListener('click', async () => {
    if (selectedIds.size === 0) return;
    if (!confirm(`确定要删除选中的 ${selectedIds.size} 张图片吗？`)) return;
    try {
      await api('/api/gallery/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ ids: Array.from(selectedIds).map(Number) })
      });
      selectedIds.clear();
      await loadGallery();
    } catch (error) {
      setStatus(error.message, 'error');
    }
  });
}

function bindGrid() {
  const grid = document.getElementById('galleryGrid');

  grid.addEventListener('change', (event) => {
    const checkbox = event.target.closest('.card-checkbox');
    if (!checkbox) return;
    const card = checkbox.closest('.gallery-card');
    if (!card) return;
    if (checkbox.checked) selectedIds.add(card.dataset.id);
    else selectedIds.delete(card.dataset.id);
    updateToolbar();
  });

  grid.addEventListener('click', (event) => {
    const deleteButton = event.target.closest('.card-delete-btn');
    if (deleteButton) {
      event.stopPropagation();
      const card = deleteButton.closest('.gallery-card');
      if (card) {
        deleteSingle(card.dataset.id);
      }
      return;
    }

    if (event.target.closest('.card-checkbox')) {
      return;
    }

    const card = event.target.closest('.gallery-card');
    if (!card) {
      return;
    }
    const image = currentGalleryItems.find((item) => String(item.id) === card.dataset.id);
    if (image) {
      openModal(image);
    }
  });
}

async function deleteSingle(id) {
  if (!confirm('确定要删除这张图片吗？')) return;
  try {
    await api(`/api/gallery/${id}`, { method: 'DELETE' });
    selectedIds.delete(String(id));
    await loadGallery();
  } catch (error) {
    setStatus(error.message, 'error');
  }
}

function openModal(image) {
  currentImage = image;
  document.getElementById('modalImage').src = image.image;
  document.getElementById('modalPrompt').textContent = image.prompt || '无提示词';
  document.getElementById('modalTask').textContent = image.task_type || 'image';
  document.getElementById('modalModel').textContent = image.model || '-';
  document.getElementById('modalRatio').textContent = image.ratio || '-';
  document.getElementById('modalResolution').textContent = image.resolution || '-';
  document.getElementById('modalCreated').textContent = new Date(image.created_at).toLocaleString();
  document.getElementById('imageModal').showModal();
}

function bindModal() {
  document.getElementById('closeModal').addEventListener('click', () => document.getElementById('imageModal').close());
  document.getElementById('deleteImage').addEventListener('click', async () => {
    if (!currentImage) return;
    if (confirm('确定要删除这张图片吗？')) {
      try {
        await api(`/api/gallery/${currentImage.id}`, { method: 'DELETE' });
        selectedIds.delete(String(currentImage.id));
        document.getElementById('imageModal').close();
        await loadGallery();
      } catch (error) {
        setStatus(error.message, 'error');
      }
    }
  });
  document.getElementById('shareImage').addEventListener('click', async () => {
    if (!currentImage) return;
    try {
      const result = await api('/api/share', { method: 'POST', body: JSON.stringify({ image_id: currentImage.id }) });
      const url = new URL(result.url, window.location.origin).toString();
      await navigator.clipboard.writeText(url).catch(() => {});
      setStatus(`分享链接已复制：${url}`, 'success');
      document.getElementById('imageModal').close();
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

initGallery().catch((error) => setStatus(error.message, 'error'));
