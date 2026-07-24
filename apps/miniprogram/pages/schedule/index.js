const api = require('../../services/api');
const { getToken, getUser, isManager, clearAuth } = require('../../utils/auth');
const { daysInMonth, weekdayLabel, isWeekend, shiftMonth } = require('../../utils/date');
const { onRealtime } = require('../../utils/realtime');

Page({
  data: {
    month: '',
    loading: false,
    saving: false,
    isManager: false,
    userName: '',
    shifts: [],
    days: [],
    rows: [],
    showPassword: false,
    passwordSaving: false,
    passwordForm: {
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    },
  },

  drafts: {},
  users: [],
  items: [],
  offRealtime: [],

  onShow() {
    const token = getToken();
    const user = getUser();
    if (!token || !user) {
      wx.reLaunch({ url: '/pages/login/index' });
      return;
    }

    const month = this.data.month || require('../../utils/date').formatMonth(new Date());
    this.setData({
      month,
      isManager: isManager(user),
      userName: user.displayName || user.username,
    });
    this.loadAll();
  },

  onLoad() {
    this.offRealtime = [
      onRealtime('schedule.updated', (payload) => {
        if (!payload || !payload.month || payload.month === this.data.month) {
          this.loadAll();
        }
      }),
      onRealtime('shifts.updated', () => this.loadAll()),
      onRealtime('users.updated', () => this.loadAll()),
    ];
  },

  onUnload() {
    (this.offRealtime || []).forEach((off) => off && off());
  },

  onPullDownRefresh() {
    this.loadAll().finally(() => wx.stopPullDownRefresh());
  },

  goShifts() {
    wx.navigateTo({ url: '/pages/shifts/index' });
  },

  goStaff() {
    wx.navigateTo({ url: '/pages/staff/index' });
  },

  openPassword() {
    this.setData({
      showPassword: true,
      passwordForm: {
        currentPassword: '',
        newPassword: '',
        confirmPassword: '',
      },
    });
  },

  closePassword() {
    this.setData({ showPassword: false });
  },

  noop() {},

  onCurrentPassword(e) {
    this.setData({ 'passwordForm.currentPassword': e.detail.value });
  },

  onNewPassword(e) {
    this.setData({ 'passwordForm.newPassword': e.detail.value });
  },

  onConfirmPassword(e) {
    this.setData({ 'passwordForm.confirmPassword': e.detail.value });
  },

  async onChangePassword() {
    const { passwordForm, passwordSaving } = this.data;
    if (passwordSaving) return;
    if (!passwordForm.currentPassword || !passwordForm.newPassword) {
      wx.showToast({ title: '请填写完整密码', icon: 'none' });
      return;
    }
    if (String(passwordForm.newPassword).length < 6) {
      wx.showToast({ title: '新密码至少 6 位', icon: 'none' });
      return;
    }
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      wx.showToast({ title: '两次新密码不一致', icon: 'none' });
      return;
    }

    this.setData({ passwordSaving: true });
    try {
      await api.changePassword({
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword,
      });
      wx.showToast({ title: '密码已修改', icon: 'success' });
      this.setData({ showPassword: false });
      setTimeout(() => {
        clearAuth();
        const app = getApp();
        if (app && app.setAuth) app.setAuth('', null);
        wx.reLaunch({ url: '/pages/login/index' });
      }, 400);
    } catch (e) {
      // toast already shown
    } finally {
      this.setData({ passwordSaving: false });
    }
  },


  prevMonth() {
    this.setData({ month: shiftMonth(this.data.month, -1) });
    this.loadAll();
  },

  nextMonth() {
    this.setData({ month: shiftMonth(this.data.month, 1) });
    this.loadAll();
  },

  buildView() {
    const days = daysInMonth(this.data.month).map((day) => ({
      workDate: day,
      dayLabel: day.slice(-2),
      weekday: weekdayLabel(day),
      weekend: isWeekend(day),
    }));

    const shiftMap = {};
    (this.data.shifts || []).forEach((s) => {
      shiftMap[s.id] = s;
    });

    const itemMap = {};
    (this.items || []).forEach((item) => {
      itemMap[`${item.userId}_${item.workDate}`] = item;
    });

    const rows = (this.users || []).map((user) => {
      const cells = days.map((day) => {
        const key = `${user.id}_${day.workDate}`;
        const base = itemMap[key];
        const draft = this.drafts[key];
        let shiftTemplateId = draft ? draft.shiftTemplateId : base && base.shiftTemplateId;
        let shift = shiftTemplateId ? shiftMap[shiftTemplateId] : null;
        return {
          workDate: day.workDate,
          weekend: day.weekend,
          shiftTemplateId: shiftTemplateId || '',
          shiftName: shift ? shift.name : base ? base.shiftName : '',
          shiftColor: shift ? shift.color : base ? base.shiftColor : '#909399',
          version: draft ? draft.version : base ? base.version : 1,
          status: draft ? draft.status : base ? base.status : 'normal',
        };
      });
      return {
        userId: user.id,
        displayName: user.displayName,
        roleLabel: user.role === 'manager' ? '店长' : '店员',
        cells,
      };
    });

    this.setData({ days, rows });
  },

  async loadAll() {
    if (this.data.loading) return;
    this.setData({ loading: true });
    try {
      const [users, shifts, schedules] = await Promise.all([
        api.users(),
        api.shifts(),
        api.schedules(this.data.month),
      ]);
      this.users = users || [];
      this.items = (schedules && schedules.items) || [];
      this.drafts = {};
      this.setData({ shifts: shifts || [] });
      this.buildView();
    } catch (e) {
      // handled
    } finally {
      this.setData({ loading: false });
    }
  },

  onCellTap(e) {
    if (!this.data.isManager) return;
    const { userId, day } = e.currentTarget.dataset;
    const shifts = this.data.shifts || [];
    if (!userId || !day || !shifts.length) return;

    const key = `${userId}_${day}`;
    const row = (this.data.rows || []).find((r) => r.userId === userId);
    const cell = row && (row.cells || []).find((c) => c.workDate === day);
    const currentId = cell && cell.shiftTemplateId;
    const currentIndex = shifts.findIndex((s) => s.id === currentId);
    const next = shifts[(currentIndex + 1 + shifts.length) % shifts.length];
    this.drafts[key] = {
      shiftTemplateId: next.id,
      version: cell && cell.version,
      status: next.isOff ? 'leave' : 'normal',
    };
    this.buildView();
  },

  async saveAll() {
    const payloadItems = Object.keys(this.drafts).map((key) => {
      const [userId, workDate] = key.split('_');
      const draft = this.drafts[key];
      return {
        userId,
        workDate,
        shiftTemplateId: draft.shiftTemplateId,
        status: draft.status,
        version: draft.version,
      };
    });

    if (!payloadItems.length) {
      wx.showToast({ title: '没有需要保存的修改', icon: 'none' });
      return;
    }

    this.setData({ saving: true });
    try {
      const data = await api.saveMonthSchedules({
        month: this.data.month,
        items: payloadItems,
      });
      this.items = (data && data.items) || this.items;
      this.drafts = {};
      this.buildView();
      wx.showToast({ title: '已保存', icon: 'success' });
    } catch (e) {
      // handled
    } finally {
      this.setData({ saving: false });
    }
  },

  onLogout() {
    clearAuth();
    const app = getApp();
    if (app && app.logout) {
      app.logout();
    } else {
      wx.reLaunch({ url: '/pages/login/index' });
    }
  },
});

