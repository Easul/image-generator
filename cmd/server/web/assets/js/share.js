async function loadShare() {
  const token = new URLSearchParams(window.location.search).get('token') || window.location.pathname.split('/').filter(Boolean).pop();
  const container = document.getElementById('shareContent');
  if (!token || token === 'share.html') {
    container.innerHTML = '<div class="empty-state"><h1>分享链接无效</h1><p>缺少 token 参数。</p></div>';
    return;
  }
  const response = await fetch(`/api/share/${encodeURIComponent(token)}`);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    container.innerHTML = `<div class="empty-state"><h1>无法打开分享</h1><p>${escapeHTML(data.error || '分享不存在或已过期')}</p></div>`;
    return;
  }
  const image = data.image;
  container.innerHTML = `<img src="${image.image}" alt="${escapeHTML(image.prompt || '分享图片')}">
    <div class="share-meta">
      <p class="eyebrow">Shared Image</p>
      <h1>${escapeHTML(image.prompt || '无提示词')}</h1>
      <p class="muted">模型：${escapeHTML(image.model || '-')} · 尺寸：${escapeHTML(image.resolution || '-')} · 创建：${new Date(image.created_at).toLocaleString()}</p>
      <a class="secondary-button" href="/">打开工作台</a>
    </div>`;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>'"]/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char]));
}

loadShare();
