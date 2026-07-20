const TOKEN_KEY = 'shift_token';
const USER_KEY = 'shift_user';

function getToken() {
  return wx.getStorageSync(TOKEN_KEY) || '';
}

function getUser() {
  try {
    return wx.getStorageSync(USER_KEY) || null;
  } catch (e) {
    return null;
  }
}

function setAuth(token, user) {
  wx.setStorageSync(TOKEN_KEY, token || '');
  wx.setStorageSync(USER_KEY, user || null);
}

function clearAuth() {
  wx.removeStorageSync(TOKEN_KEY);
  wx.removeStorageSync(USER_KEY);
}

function isManager(user) {
  return !!(user && user.role === 'manager');
}

module.exports = {
  TOKEN_KEY,
  USER_KEY,
  getToken,
  getUser,
  setAuth,
  clearAuth,
  isManager,
};
