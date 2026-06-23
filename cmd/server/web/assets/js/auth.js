async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}

function setStatus(message, type = '') {
  const status = document.getElementById('authStatus');
  if (!status) return;
  status.textContent = message;
  status.className = `status-line ${type}`;
}

async function loadConfig() {
  try {
    const config = await requestJSON('/api/config');
    document.querySelectorAll('.brand span:last-child').forEach((item) => { item.textContent = config.site_name || 'AI 图片工作台'; });
    document.querySelectorAll('.brand-mark').forEach((item) => { item.textContent = config.site_icon || 'AI'; });
    if (document.body.dataset.page === 'register' && config.allow_register === false) {
      setStatus('当前系统已关闭注册。第一个用户仍可由空库启动时创建。', 'error');
    }
  } catch (_) {}
}

function bindAuthForm(formId, endpoint) {
  const form = document.getElementById(formId);
  if (!form) return;
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = form.querySelector('button[type="submit"]');
    const payload = Object.fromEntries(new FormData(form).entries());
    button.disabled = true;
    setStatus('正在处理...');
    try {
      await requestJSON(endpoint, { method: 'POST', body: JSON.stringify(payload) });
      setStatus('成功，正在进入工作台...', 'success');
      window.location.href = '/';
    } catch (error) {
      setStatus(error.message, 'error');
    } finally {
      button.disabled = false;
    }
  });
}

loadConfig();
bindAuthForm('loginForm', '/api/login');
bindAuthForm('registerForm', '/api/register');
