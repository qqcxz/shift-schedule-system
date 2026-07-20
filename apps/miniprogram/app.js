const { getToken, getUser, clearAuth } = require('./utils/auth');
const { connectSocket, disconnectSocket } = require('./utils/realtime');

const API_BASE_KEY = 'shift_api_base';

App({
  globalData: {
    // 开发默认指向本机后端；真机调试请改成电脑局域网 IP
    // 例如：http://192.168.1.8:3000
    apiBaseUrl: 'http://127.0.0.1:3000',
    token: '',
    user: null,
  },

  onLaunch() {
    const savedBase = wx.getStorageSync(API_BASE_KEY);
    if (savedBase) {
      this.globalData.apiBaseUrl = String(savedBase).replace(/\/$/, '');
    }

    const token = getToken();
    const user = getUser();
    this.globalData.token = token || '';
    this.globalData.user = user || null;

    if (token) {
      connectSocket(token);
    }
  },

  setApiBaseUrl(url) {
    const value = String(url || 'http://127.0.0.1:3000').replace(/\/$/, '');
    this.globalData.apiBaseUrl = value;
    wx.setStorageSync(API_BASE_KEY, value);
  },

  setAuth(token, user) {
    this.globalData.token = token || '';
    this.globalData.user = user || null;
    if (token) {
      connectSocket(token);
    } else {
      disconnectSocket();
    }
  },

  logout() {
    clearAuth();
    this.setAuth('', null);
    wx.reLaunch({ url: '/pages/login/index' });
  },
});
