const api = require('../../services/api');
const { getToken, getUser, isManager } = require('../../utils/auth');
const { onRealtime } = require('../../utils/realtime');

const ROLE_OPTIONS = [
  { label: '店员', value: 'staff' },
  { label: '店长', value: 'manager' },
];

Page({
  data: {
    loading: false,
    saving: false,
    includeInactive: true,
    users: [],
    showDialog: false,
    editingId: '',
    roleNames: ROLE_OPTIONS.map((item) => item.label),
    roleIndex: 0,
    form: {
      displayName: '',
      username: '',
      password: '',
      role: 'staff',
    },
  },

  offRealtime: [],

  onShow() {
    const token = getToken();
    const user = getUser();
    if (!token || !user) {
      wx.reLaunch({ url: '/pages/login/index' });
      return;
    }
    if (!isManager(user)) {
      wx.showToast({ title: '仅店长可管理员工', icon: 'none' });
      setTimeout(() => wx.navigateBack({ fail: () => wx.switchTab({ url: '/pages/schedule/index' }) }), 500);
      return;
    }
    this.loadUsers();
  },

  onLoad() {
    this.offRealtime = [
      onRealtime('users.updated', () => this.loadUsers()),
    ];
  },

  onUnload() {
    (this.offRealtime || []).forEach((off) => off && off());
  },

  onPullDownRefresh() {
    this.loadUsers().finally(() => wx.stopPullDownRefresh());
  },

  onIncludeInactive(e) {
    this.setData({ includeInactive: !!e.detail.value });
    this.loadUsers();
  },

  async loadUsers() {
    this.setData({ loading: true });
    try {
      const users = await api.users(this.data.includeInactive);
      this.setData({ users: users || [] });
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ loading: false });
    }
  },

  openCreate() {
    this.setData({
      showDialog: true,
      editingId: '',
      roleIndex: 0,
      form: {
        displayName: '',
        username: '',
        password: '',
        role: 'staff',
      },
    });
  },

  openEdit(e) {
    const id = e.currentTarget.dataset.id;
    const row = (this.data.users || []).find((item) => item.id === id);
    if (!row) return;
    const roleIndex = Math.max(0, ROLE_OPTIONS.findIndex((item) => item.value === row.role));
    this.setData({
      showDialog: true,
      editingId: row.id,
      roleIndex,
      form: {
        displayName: row.displayName,
        username: row.username,
        password: '',
        role: row.role,
      },
    });
  },

  closeDialog() {
    this.setData({ showDialog: false });
  },

  noop() {},

  onDisplayName(e) {
    this.setData({ 'form.displayName': e.detail.value });
  },
  onUsername(e) {
    this.setData({ 'form.username': e.detail.value });
  },
  onPassword(e) {
    this.setData({ 'form.password': e.detail.value });
  },
  onRoleChange(e) {
    const roleIndex = Number(e.detail.value || 0);
    this.setData({
      roleIndex,
      'form.role': ROLE_OPTIONS[roleIndex].value,
    });
  },

  async onSave() {
    const { form, editingId, saving } = this.data;
    if (saving) return;
    if (!form.displayName || !String(form.displayName).trim() || !form.username || !String(form.username).trim()) {
      wx.showToast({ title: '请填写姓名和账号', icon: 'none' });
      return;
    }
    if (!editingId && String(form.password || '').length < 6) {
      wx.showToast({ title: '密码至少 6 位', icon: 'none' });
      return;
    }
    if (editingId && form.password && String(form.password).length < 6) {
      wx.showToast({ title: '新密码至少 6 位', icon: 'none' });
      return;
    }

    this.setData({ saving: true });
    try {
      if (editingId) {
        await api.updateUser(editingId, {
          displayName: String(form.displayName).trim(),
          username: String(form.username).trim(),
          role: form.role,
          password: form.password || undefined,
        });
        wx.showToast({ title: '员工已更新', icon: 'success' });
      } else {
        await api.createUser({
          displayName: String(form.displayName).trim(),
          username: String(form.username).trim(),
          password: form.password,
          role: form.role,
        });
        wx.showToast({ title: '员工已创建', icon: 'success' });
      }
      this.setData({ showDialog: false });
      await this.loadUsers();
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ saving: false });
    }
  },

  async onToggleActive(e) {
    const id = e.currentTarget.dataset.id;
    const isActive = e.currentTarget.dataset.active === '1' || e.currentTarget.dataset.active === 1;
    try {
      await api.updateUser(id, { isActive: !!isActive });
      wx.showToast({ title: isActive ? '已启用' : '已停用', icon: 'success' });
      this.loadUsers();
    } catch (err) {
      // toast already shown
    }
  },

  onDelete(e) {
    const { id, name } = e.currentTarget.dataset;
    wx.showModal({
      title: '删除员工',
      content: `确认删除员工「${name}」？删除后将停用该账号，历史排班仍会保留。`,
      success: async (res) => {
        if (!res.confirm) return;
        try {
          await api.deleteUser(id);
          wx.showToast({ title: '已删除', icon: 'success' });
          this.loadUsers();
        } catch (err) {
          // toast already shown
        }
      },
    });
  },
});
