const api = require('../../services/api');
const { setAuth, getToken, getUser } = require('../../utils/auth');

Page({
  data: {
    username: '',
    password: '',
    loading: false,
    apiBaseUrl: 'http://127.0.0.1:3000',
  },

  onShow() {
    const app = getApp();
    this.setData({
      apiBaseUrl: (app.globalData && app.globalData.apiBaseUrl) || 'http://127.0.0.1:3000',
    });

    const token = getToken();
    const user = getUser();
    if (token && user) {
      wx.switchTab({ url: '/pages/schedule/index' });
    }
  },

  onUsername(e) {
    this.setData({ username: e.detail.value });
  },

  onPassword(e) {
    this.setData({ password: e.detail.value });
  },

  onApiBase(e) {
    const apiBaseUrl = (e.detail.value || '').trim().replace(/\/$/, '');
    this.setData({ apiBaseUrl });
    const app = getApp();
    if (app && app.setApiBaseUrl) {
      app.setApiBaseUrl(apiBaseUrl);
    } else if (app && app.globalData) {
      app.globalData.apiBaseUrl = apiBaseUrl;
    }
  },

  async onSubmit() {
    const { username, password, apiBaseUrl, loading } = this.data;
    if (loading) return;
    if (!username || !password) {
      wx.showToast({ title: '请输入用户名和密码', icon: 'none' });
      return;
    }

    const app = getApp();
    if (app && app.setApiBaseUrl) {
      app.setApiBaseUrl(apiBaseUrl || 'http://127.0.0.1:3000');
    } else if (app && app.globalData) {
      app.globalData.apiBaseUrl = (apiBaseUrl || 'http://127.0.0.1:3000').replace(/\/$/, '');
    }

    this.setData({ loading: true });
    try {
      const data = await api.login(String(username).trim(), password);
      setAuth(data.accessToken, data.user);
      if (app && app.setAuth) {
        app.setAuth(data.accessToken, data.user);
      }
      wx.showToast({ title: '登录成功', icon: 'success' });
      setTimeout(() => {
        wx.switchTab({ url: '/pages/schedule/index' });
      }, 300);
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ loading: false });
    }
  },
});
