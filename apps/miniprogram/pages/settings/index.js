const api = require('../../services/api');
const { getToken, getUser, isManager } = require('../../utils/auth');

const EVENT_LABELS = {
  schedule_publish: '排班发布',
  schedule_request: '排班申请',
  swap_request: '换班申请',
  pre_shift_reminder: '班前提醒',
  test: '测试消息',
};

const STATE_LABELS = {
  pending: '待发送',
  preparing: '准备发送',
  sending: '发送中',
  sent: '已发送',
  failed: '失败',
  rate_limited: '限流等待',
};

Page({
  data: {
    loading: false,
    saving: false,
    testing: false,
    removing: false,
    settingsSaving: false,
    settingsDirty: false,
    isManager: false,
    token: '',
    binding: { provider: '虾推啥', tokenBound: false, maskedToken: '' },
    settings: {
      schedulePublishEnabled: false,
      swapRequestEnabled: false,
      preShiftReminderEnabled: false,
      preShiftReminderTime: '08:00',
      timezone: 'Asia/Shanghai',
    },
    deliveries: [],
  },

  onLoad() {
    const token = getToken();
    const user = getUser();
    if (!token || !user) {
      wx.reLaunch({ url: '/pages/login/index' });
      return;
    }
    this.setData({ isManager: isManager(user) });
  },

  onShow() {
    if (getToken()) this.loadAll();
  },

  onPullDownRefresh() {
    this.loadAll().finally(() => wx.stopPullDownRefresh());
  },

  async loadAll() {
    this.setData({ loading: true });
    try {
      const jobs = [api.myPushBinding(), api.pushDeliveries()];
      if (this.data.isManager) jobs.push(api.storePushSettings());
      const result = await Promise.all(jobs);
      const binding = result[0] || this.data.binding;
      const deliveries = (result[1] || []).map((item) => ({
        ...item,
        eventLabel: EVENT_LABELS[item.eventType] || item.eventType,
        stateLabel: STATE_LABELS[item.state] || item.state,
        stateClass: item.state === 'sent' ? 'success' : item.state === 'failed' ? 'danger' : item.state === 'rate_limited' ? 'warning' : 'info',
        timeLabel: item.createdAt ? String(item.createdAt).replace('T', ' ').slice(0, 16) : '',
      }));
      const data = { binding, deliveries };
      if (this.data.isManager && result[2]) {
        const settings = result[2];
        data.settings = settings;
      }
      this.setData(data);
    } catch (e) {
      // request layer already shows a toast
    } finally {
      this.setData({ loading: false });
    }
  },

  onTokenInput(e) {
    this.setData({ token: e.detail.value });
  },

  async saveBinding() {
    const token = String(this.data.token || '').trim();
    if (!token) {
      wx.showToast({ title: '请粘贴虾推啥 Token', icon: 'none' });
      return;
    }
    this.setData({ saving: true });
    try {
      const binding = await api.saveMyPushBinding(token);
      this.setData({ binding, token: '' });
      wx.showToast({ title: 'Token 已保存', icon: 'success' });
    } catch (e) {
      // handled
    } finally {
      this.setData({ saving: false });
    }
  },

  testBinding() {
    this.setData({ testing: true });
    api.testMyPushBinding()
      .then((delivery) => {
        wx.showToast({ title: delivery && delivery.state === 'sent' ? '测试消息已发送' : '请查看推送记录', icon: 'none' });
        return this.loadAll();
      })
      .catch(() => this.loadAll())
      .finally(() => this.setData({ testing: false }));
  },

  removeBinding() {
    wx.showModal({
      title: '确认解绑',
      content: '解绑后将不会再收到虾推啥微信通知。',
      success: async (result) => {
        if (!result.confirm) return;
        this.setData({ removing: true });
        try {
          await api.deleteMyPushBinding();
          this.setData({ binding: { provider: '虾推啥', tokenBound: false, maskedToken: '' }, token: '' });
          wx.showToast({ title: '已解绑', icon: 'success' });
        } catch (e) {
          // handled
        } finally {
          this.setData({ removing: false });
        }
      },
    });
  },

  onSchedulePublishChange(e) {
    this.setData({ 'settings.schedulePublishEnabled': e.detail.value, settingsDirty: true });
    this.saveStoreSettings();
  },

  onSwapRequestChange(e) {
    this.setData({ 'settings.swapRequestEnabled': e.detail.value, settingsDirty: true });
    this.saveStoreSettings();
  },

  onPreShiftChange(e) {
    this.setData({ 'settings.preShiftReminderEnabled': e.detail.value, settingsDirty: true });
    this.saveStoreSettings();
  },

  onReminderTimeChange(e) {
    this.setData({
      settingsDirty: true,
      'settings.preShiftReminderTime': e.detail.value,
    });
    this.saveStoreSettings();
  },

  async saveStoreSettings() {
    if (!this.data.isManager) return;
    if (this.data.settingsSaving) return;
    this.setData({ settingsSaving: true, settingsDirty: false });
    const snapshot = { ...this.data.settings };
    try {
      await api.updateStorePushSettings(snapshot);
      wx.showToast({ title: '设置已保存', icon: 'success' });
    } catch (e) {
      await this.loadAll();
      this.setData({ settingsDirty: false });
    } finally {
      const dirty = this.data.settingsDirty;
      this.setData({ settingsSaving: false });
      if (dirty) this.saveStoreSettings();
    }
  },

  async queryStatus(e) {
    const id = e.currentTarget.dataset.id;
    if (!id) return;
    try {
      const item = await api.queryPushDeliveryStatus(id);
      wx.showModal({
        title: '投递状态',
        content: (item && item.providerStatus) || '服务商暂未返回明确状态',
        showCancel: false,
      });
      this.loadAll();
    } catch (err) {
      // handled
    }
  },
});
