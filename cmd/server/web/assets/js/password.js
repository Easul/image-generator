async function api(path, options = {}) {
  const response = await fetch(path, { credentials: 'include', headers: { 'Content-Type': 'application/json' }, ...options });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) window.location.href = '/login.html';
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}

function setStatus(message, type = '') {
  const status = document.getElementById('passwordStatus');
  status.textContent = message || '';
  status.className = `status-line ${type}`;
}

async function initPasswordPage() {
  const [profile, config] = await Promise.all([api('/api/profile'), api('/api/config')]);
  document.getElementById('siteName').textContent = config.site_name || 'AI 图片工作台';
  document.getElementById('siteIcon').textContent = config.site_icon || 'AI';
  document.getElementById('currentUser').textContent = profile.user.username;
  document.getElementById('adminBadge').classList.toggle('hidden', !profile.user.is_admin);
  document.querySelectorAll('.admin-link').forEach((item) => {
    item.classList.toggle('hidden', !profile.user.is_admin);
  });
  bindPasswordForm();
  bindLogout();
}

function bindPasswordForm() {
  const form = document.getElementById('passwordForm');
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const submitButton = form.querySelector('button[type="submit"]');
    submitButton.disabled = true;
    try {
      await api('/api/profile/password', { method: 'POST', body: JSON.stringify(Object.fromEntries(new FormData(form).entries())) });
      form.reset();
      setStatus('密码已更新，请使用新密码登录。', 'success');
    } catch (error) {
      setStatus(error.message, 'error');
    } finally {
      submitButton.disabled = false;
    }
  });
}

function bindLogout() {
  document.getElementById('logoutBtn').addEventListener('click', async () => {
    await api('/api/logout', { method: 'POST', body: '{}' }).catch(() => {});
    window.location.href = '/login.html';
  });
}

initPasswordPage().catch((error) => setStatus(error.message, 'error'));
