const { getToken, clearAuth } = require('./auth');

function getBaseUrl() {
  const app = getApp();
  return (app && app.globalData && app.globalData.apiBaseUrl) || 'http://127.0.0.1:3000';
}

function buildUrl(path, params) {
  const base = getBaseUrl().replace(/\/$/, '');
  const cleanPath = path.startsWith('/') ? path : `/${path}`;
  let url = `${base}/api${cleanPath}`;
  if (params && typeof params === 'object') {
    const qs = Object.keys(params)
      .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== '')
      .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(params[key])}`)
      .join('&');
    if (qs) url += `?${qs}`;
  }
  return url;
}

function request(options) {
  const {
    url,
    method = 'GET',
    data,
    params,
    header = {},
    showError = true,
    auth = true,
  } = options;

  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...header,
  };
  if (auth && token) {
    headers.Authorization = `Bearer ${token}`;
  }

  return new Promise((resolve, reject) => {
    wx.request({
      url: buildUrl(url, params),
      method,
      data,
      header: headers,
      timeout: 15000,
      success(res) {
        const status = res.statusCode || 0;
        if (status >= 200 && status < 300) {
          resolve(res.data);
          return;
        }

        if (status === 401) {
          clearAuth();
          const app = getApp();
          if (app && app.setAuth) app.setAuth('', null);
          wx.reLaunch({ url: '/pages/login/index' });
        }

        const message =
          (res.data && (res.data.message || res.data.error)) ||
          `请求失败(${status})`;
        const err = new Error(Array.isArray(message) ? message.join(', ') : message);
        if (showError) {
          wx.showToast({ title: err.message, icon: 'none' });
        }
        reject(err);
      },
      fail(error) {
        const err = new Error((error && error.errMsg) || '网络异常，请检查后端地址与合法域名');
        if (showError) {
          wx.showToast({ title: err.message, icon: 'none', duration: 2500 });
        }
        reject(err);
      },
    });
  });
}

module.exports = {
  request,
  getBaseUrl,
  buildUrl,
};
