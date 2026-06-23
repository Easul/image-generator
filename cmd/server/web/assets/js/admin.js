let currentUserId = 0;

async function api(path, options = {}) {
  const response = await fetch(path, { credentials: 'include', headers: { 'Content-Type': 'application/json' }, ...options });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) window.location.href = '/login.html';
  if (response.status === 403) window.location.href = '/';
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}

function setStatus(message, type = '') {
  const status = document.getElementById('adminStatus');
  status.textContent = message || '';
  status.className = `status-line ${type}`;
}

async function initAdmin() {
  const profile = await api('/api/profile');
  if (!profile.user.is_admin) window.location.href = '/';
  currentUserId = Number(profile.user.id || 0);
  document.getElementById('currentUser').textContent = profile.user.username;
  document.getElementById('adminBadge').classList.toggle('hidden', !profile.user.is_admin);
  bindLogout();
  bindForms();
  bindApiKeys();
  await Promise.all([loadSettings(), loadUsers(), loadApiKeys()]);
}

async function loadSettings() {
  const settings = await api('/api/admin/settings');
  const form = document.getElementById('settingsForm');
  form.site_name.value = settings.site_name || '';
  form.site_icon.value = settings.site_icon || 'AI';
  form.allow_register.checked = Boolean(settings.allow_register);
  form.base_url.value = settings.base_url || '';
  form.api_key.value = settings.api_key || '';
  document.querySelector('#modelsForm textarea[name="models"]').value = (settings.models || []).join('\n');
}

async function loadUsers() {
  const data = await api('/api/admin/users');
  document.getElementById('usersTable').innerHTML = (data.users || []).map((user) => `<tr>
    <td>${user.id}</td>
    <td>${escapeHTML(user.username)}</td>
    <td>${new Date(user.created_at).toLocaleString()}</td>
    <td>${user.is_admin ? '是' : '否'}</td>
    <td>${user.banned ? '已封禁' : '正常'}</td>
    <td>${user.calls_today || 0}</td>
    <td>${user.calls_total || 0}</td>
    <td>${renderUserActions(user)}</td>
  </tr>`).join('');
  document.querySelectorAll('[data-user]').forEach((button) => {
    button.addEventListener('click', async () => {
      const action = button.dataset.action;
      let endpoint;
      let successMessage;
      switch (action) {
        case 'admin':
          endpoint = '/api/admin/user/admin';
          successMessage = '已设为管理员';
          break;
        case 'unadmin':
          endpoint = '/api/admin/user/unadmin';
          successMessage = '已取消管理员';
          break;
        case 'ban':
          endpoint = '/api/admin/user/ban';
          successMessage = '已封禁用户';
          break;
        case 'unban':
          endpoint = '/api/admin/user/unban';
          successMessage = '已解封用户';
          break;
        case 'reset-password': {
          if (!confirm('确定要重置这个用户的密码吗？系统会生成一个随机新密码。')) return;
          try {
            const result = await api('/api/admin/user/reset-password', { method: 'POST', body: JSON.stringify({ user_id: Number(button.dataset.user) }) });
            await navigator.clipboard.writeText(result.password).catch(() => {});
            window.prompt('随机密码已生成，已尝试复制到剪贴板。请立即复制并发给用户：', result.password);
            setStatus('密码已重置', 'success');
            return;
          } catch (error) {
            setStatus(error.message, 'error');
            return;
          }
        }
        default:
          return;
      }
      try {
        await api(endpoint, { method: 'POST', body: JSON.stringify({ user_id: Number(button.dataset.user) }) });
        setStatus(successMessage, 'success');
        await loadUsers();
      } catch (error) {
        setStatus(error.message, 'error');
      }
    });
  });
}

function renderUserActions(user) {
  if (Number(user.id) === currentUserId) {
    return '<div class="user-actions"><span class="muted-text">当前账户</span></div>';
  }
  return `
    <div class="user-actions">
      <button class="secondary-button user-reset-button" data-user="${user.id}" data-action="reset-password">重置密码</button>
      <button class="secondary-button" data-user="${user.id}" data-action="${user.is_admin ? 'unadmin' : 'admin'}">${user.is_admin ? '取消管理员' : '设为管理员'}</button>
      <button class="secondary-button ${user.banned ? '' : 'danger'}" data-user="${user.id}" data-action="${user.banned ? 'unban' : 'ban'}">${user.banned ? '解封' : '封禁'}</button>
    </div>
  `;
}

function bindForms() {
  document.getElementById('settingsForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const payload = {
      site_name: form.site_name.value,
      site_icon: form.site_icon.value,
      allow_register: form.allow_register.checked,
      base_url: form.base_url.value,
      api_key: form.api_key.value,
    };
    try {
      await api('/api/admin/settings', { method: 'POST', body: JSON.stringify(payload) });
      setStatus('设置已保存', 'success');
    } catch (error) {
      setStatus(error.message, 'error');
    }
  });

  document.getElementById('modelsForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const models = event.currentTarget.models.value.split('\n').map((item) => item.trim()).filter(Boolean);
    try {
      await api('/api/admin/models', { method: 'POST', body: JSON.stringify({ models }) });
      setStatus('模型列表已保存', 'success');
    } catch (error) {
      setStatus(error.message, 'error');
    }
  });

  document.getElementById('testApi').addEventListener('click', async () => {
    const form = document.getElementById('settingsForm');
    try {
      await api('/api/admin/test-api', { method: 'POST', body: JSON.stringify({ base_url: form.base_url.value, api_key: form.api_key.value }) });
      setStatus('连接成功，/models 可访问。', 'success');
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

async function loadApiKeys() {
  const data = await api('/api/admin/api-keys');
  const tbody = document.getElementById('apiKeysTable');
  if (!data.keys || data.keys.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="muted-text" style="text-align:center;">暂无 API 密钥</td></tr>';
    return;
  }

  // Store full keys in a global object for copying
  window.apiKeysMap = {};
  data.keys.forEach((key) => {
    if (key.key) {
      window.apiKeysMap[key.id] = key.key;
    }
  });

  tbody.innerHTML = data.keys.map((key) => `<tr>
    <td>
      <div style="display:flex;align-items:center;gap:8px;">
        <code style="font-size:13px;">${escapeHTML(key.masked_key)}</code>
        ${key.key ? `<button class="icon-button copy-key-btn" data-key-id="${key.id}" title="复制完整密钥">📋</button>` : ''}
      </div>
    </td>
    <td>${escapeHTML(key.note || '-')}</td>
    <td>${key.last_used_at || '未使用'}</td>
    <td>${key.created_at}</td>
    <td><button class="secondary-button danger delete-key-btn" data-key-id="${key.id}">删除</button></td>
  </tr>`).join('');

  document.querySelectorAll('.delete-key-btn').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (!confirm('确定要删除这个 API 密钥吗？删除后无法恢复。')) return;
      try {
        await api('/api/admin/api-keys', { method: 'DELETE', body: JSON.stringify({ id: Number(btn.dataset.keyId) }) });
        setStatus('密钥已删除', 'success');
        await loadApiKeys();
      } catch (error) {
        setStatus(error.message, 'error');
      }
    });
  });

  document.querySelectorAll('.copy-key-btn').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const keyId = Number(btn.dataset.keyId);
      const fullKey = window.apiKeysMap[keyId];
      if (!fullKey) {
        setStatus('密钥不存在', 'error');
        return;
      }
      try {
        await navigator.clipboard.writeText(fullKey);
        setStatus('密钥已复制到剪贴板', 'success');
      } catch (error) {
        window.prompt('无法自动复制，请手动复制以下密钥：', fullKey);
      }
    });
  });
}

function bindApiKeys() {
  document.getElementById('createApiKeyBtn').addEventListener('click', async () => {
    const note = prompt('请输入备注（可选）：');
    if (note === null) return;
    try {
      const result = await api('/api/admin/api-keys', { method: 'POST', body: JSON.stringify({ note: note.trim() }) });

      // Try to copy to clipboard first
      let copySuccess = false;
      try {
        await navigator.clipboard.writeText(result.key);
        copySuccess = true;
      } catch (e) {
        // Clipboard API failed
      }

      // Show the key prominently
      const message = copySuccess
        ? `API 密钥已生成并复制到剪贴板！\n\n请妥善保存（仅显示一次）：\n\n${result.key}`
        : `API 密钥已生成！\n\n请复制并妥善保存（仅显示一次）：\n\n${result.key}`;

      alert(message);

      setStatus(copySuccess ? '密钥已创建并复制' : '密钥已创建', 'success');
      await loadApiKeys();
    } catch (error) {
      setStatus(error.message, 'error');
    }
  });
}

initAdmin().catch((error) => setStatus(error.message, 'error'));
