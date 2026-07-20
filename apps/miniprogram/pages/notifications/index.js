const api = require('../../services/api');
const { getToken, getUser } = require('../../utils/auth');
const { formatDateTime } = require('../../utils/date');
const { onRealtime } = require('../../utils/realtime');

Page({
  data: {
    loading: false,
    items: [],
  },

  offRealtime: [],

  onShow() {
    const token = getToken();
    const user = getUser();
    if (!token || !user) {
      wx.reLaunch({ url: '/pages/login/index' });
      return;
    }
    this.load();
  },

  onLoad() {
    this.offRealtime = [
      onRealtime('notification.created', () => this.load()),
    ];
  },

  onUnload() {
    (this.offRealtime || []).forEach((off) => off && off());
  },

  onPullDownRefresh() {
    this.load().finally(() => wx.stopPullDownRefresh());
  },

  async load() {
    this.setData({ loading: true });
    try {
      const list = await api.notifications();
      const items = (list || []).map((item) => ({
        ...item,
        createdAtText: formatDateTime(item.createdAt),
      }));
      this.setData({ items });
    } catch (e) {
      // handled
    } finally {
      this.setData({ loading: false });
    }
  },

  async markOne(e) {
    const { id, read } = e.currentTarget.dataset;
    if (read || !id) return;
    try {
      await api.markNotificationRead(id);
      const items = (this.data.items || []).map((item) =>
        item.id === id ? { ...item, isRead: true } : item,
      );
      this.setData({ items });
    } catch (err) {
      // handled
    }
  },

  async markAll() {
    try {
      await api.markAllNotificationsRead();
      const items = (this.data.items || []).map((item) => ({ ...item, isRead: true }));
      this.setData({ items });
      wx.showToast({ title: '已全部已读', icon: 'success' });
    } catch (e) {
      // handled
    }
  },
});
