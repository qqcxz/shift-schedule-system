const api = require('../../services/api');
const { getToken, getUser, isManager } = require('../../utils/auth');
const { onRealtime } = require('../../utils/realtime');

const COLOR_OPTIONS = [
  { name: '绿色', value: '#67C23A' },
  { name: '橙色', value: '#E6A23C' },
  { name: '蓝色', value: '#409EFF' },
  { name: '红色', value: '#F56C6C' },
  { name: '灰色', value: '#909399' },
  { name: '紫色', value: '#8B5CF6' },
];

Page({
  data: {
    loading: false,
    saving: false,
    shifts: [],
    showDialog: false,
    editingId: '',
    colorNames: COLOR_OPTIONS.map((item) => item.name),
    colorIndex: 2,
    form: {
      name: '',
      code: '',
      startTime: '09:00',
      endTime: '13:00',
      color: '#409EFF',
      sortOrder: 1,
      isOff: false,
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
      wx.showToast({ title: '仅店长可管理班次', icon: 'none' });
      setTimeout(() => wx.navigateBack({ fail: () => wx.switchTab({ url: '/pages/schedule/index' }) }), 500);
      return;
    }
    this.loadShifts();
  },

  onLoad() {
    this.offRealtime = [
      onRealtime('shifts.updated', () => this.loadShifts()),
    ];
  },

  onUnload() {
    (this.offRealtime || []).forEach((off) => off && off());
  },

  onPullDownRefresh() {
    this.loadShifts().finally(() => wx.stopPullDownRefresh());
  },

  async loadShifts() {
    this.setData({ loading: true });
    try {
      const shifts = await api.shifts();
      this.setData({ shifts: shifts || [] });
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ loading: false });
    }
  },

  openCreate() {
    const last = (this.data.shifts || [])[this.data.shifts.length - 1];
    this.setData({
      showDialog: true,
      editingId: '',
      colorIndex: 2,
      form: {
        name: '',
        code: '',
        startTime: '09:00',
        endTime: '13:00',
        color: '#409EFF',
        sortOrder: (last && last.sortOrder ? last.sortOrder : 0) + 1,
        isOff: false,
      },
    });
  },

  openEdit(e) {
    const id = e.currentTarget.dataset.id;
    const row = (this.data.shifts || []).find((item) => item.id === id);
    if (!row) return;
    const colorIndex = Math.max(
      0,
      COLOR_OPTIONS.findIndex((item) => item.value.toLowerCase() === String(row.color || '').toLowerCase()),
    );
    this.setData({
      showDialog: true,
      editingId: row.id,
      colorIndex: colorIndex < 0 ? 2 : colorIndex,
      form: {
        name: row.name,
        code: row.code,
        startTime: row.startTime,
        endTime: row.endTime,
        color: row.color,
        sortOrder: row.sortOrder || 0,
        isOff: !!row.isOff,
      },
    });
  },

  closeDialog() {
    this.setData({ showDialog: false });
  },

  noop() {},

  onName(e) {
    this.setData({ 'form.name': e.detail.value });
  },
  onCode(e) {
    this.setData({ 'form.code': e.detail.value });
  },
  onIsOff(e) {
    this.setData({ 'form.isOff': !!e.detail.value });
  },
  onStartTime(e) {
    this.setData({ 'form.startTime': e.detail.value });
  },
  onEndTime(e) {
    this.setData({ 'form.endTime': e.detail.value });
  },
  onColorChange(e) {
    const colorIndex = Number(e.detail.value || 0);
    this.setData({
      colorIndex,
      'form.color': COLOR_OPTIONS[colorIndex].value,
    });
  },
  onSortOrder(e) {
    this.setData({ 'form.sortOrder': Number(e.detail.value || 0) });
  },

  async onSave() {
    const { form, editingId, saving } = this.data;
    if (saving) return;
    if (!form.name || !String(form.name).trim()) {
      wx.showToast({ title: '请填写班次名称', icon: 'none' });
      return;
    }
    if (!form.isOff && (!form.startTime || !form.endTime)) {
      wx.showToast({ title: '请填写班次时间', icon: 'none' });
      return;
    }

    this.setData({ saving: true });
    try {
      const payload = {
        name: String(form.name).trim(),
        code: form.code ? String(form.code).trim() : undefined,
        startTime: form.isOff ? '00:00' : form.startTime,
        endTime: form.isOff ? '00:00' : form.endTime,
        color: form.color || '#409EFF',
        sortOrder: Number(form.sortOrder || 0),
        isOff: !!form.isOff,
      };
      if (editingId) {
        await api.updateShift(editingId, payload);
        wx.showToast({ title: '班次已更新', icon: 'success' });
      } else {
        await api.createShift(payload);
        wx.showToast({ title: '班次已创建', icon: 'success' });
      }
      this.setData({ showDialog: false });
      await this.loadShifts();
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ saving: false });
    }
  },

  onDelete(e) {
    const { id, name } = e.currentTarget.dataset;
    wx.showModal({
      title: '删除班次',
      content: `确认删除班次「${name}」？若已被排班使用将无法删除。`,
      success: async (res) => {
        if (!res.confirm) return;
        try {
          await api.deleteShift(id);
          wx.showToast({ title: '已删除', icon: 'success' });
          this.loadShifts();
        } catch (err) {
          // toast already shown
        }
      },
    });
  },
});
