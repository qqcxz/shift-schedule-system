const api = require('../../services/api');
const { getToken, getUser, isManager } = require('../../utils/auth');
const { formatDate, formatDateTime, typeLabel, statusLabel, statusClass } = require('../../utils/date');
const { onRealtime } = require('../../utils/realtime');

Page({
  data: {
    loading: false,
    submitting: false,
    isManager: false,
    status: '',
    statusOptions: [
      { label: '全部', value: '' },
      { label: '待审批', value: 'pending' },
      { label: '已通过', value: 'approved' },
      { label: '已驳回', value: 'rejected' },
    ],
    requests: [],
    showCreate: false,
    form: {
      type: 'leave',
      fromDate: '',
      targetUserId: '',
      toShiftId: '',
      reason: '',
    },
    staffList: [],
    staffNames: [],
    targetIndex: 0,
    shifts: [],
    shiftNames: [],
    shiftIndex: 0,
  },

  offRealtime: [],

  onShow() {
    const token = getToken();
    const user = getUser();
    if (!token || !user) {
      wx.reLaunch({ url: '/pages/login/index' });
      return;
    }

    const manager = isManager(user);
    this.setData({
      isManager: manager,
      status: manager ? 'pending' : this.data.status,
      'form.fromDate': this.data.form.fromDate || formatDate(new Date()),
    });
    Promise.resolve(this.loadMeta()).then(() => this.loadRequests());
  },

  onLoad() {
    this.offRealtime = [
      onRealtime('request.created', () => this.loadRequests()),
      onRealtime('request.resolved', () => this.loadRequests()),
    ];
  },

  onUnload() {
    (this.offRealtime || []).forEach((off) => off && off());
  },

  onPullDownRefresh() {
    this.loadRequests().finally(() => wx.stopPullDownRefresh());
  },

  onStatusChange(e) {
    this.setData({ status: e.currentTarget.dataset.value || '' });
    this.loadRequests();
  },

  async loadMeta() {
    try {
      const user = getUser();
      const [users, shifts] = await Promise.all([api.users(), api.shifts()]);
      const staffList = (users || []).filter((u) => u.role === 'staff' && u.id !== (user && user.id));
      this.setData({
        staffList,
        staffNames: staffList.map((u) => u.displayName),
        shifts: shifts || [],
        shiftNames: (shifts || []).map((s) => s.name),
        targetIndex: 0,
        shiftIndex: 0,
      });
    } catch (e) {
      // handled
    }
  },

  async loadRequests() {
    this.setData({ loading: true });
    try {
      const list = await api.requests(this.data.status || undefined);
      const shiftMap = {};
      (this.data.shifts || []).forEach((s) => {
        shiftMap[s.id] = s.name;
      });
      const requests = (list || []).map((item) => ({
        ...item,
        typeLabel: typeLabel(item.type),
        statusLabel: statusLabel(item.status),
        statusClass: statusClass(item.status),
        requesterName: (item.requester && item.requester.displayName) || item.requesterId,
        targetName: (item.targetUser && item.targetUser.displayName) || item.targetUserId || '-',
        toShiftName: shiftMap[item.toShiftId] || item.toShiftId || '-',
        createdAtText: formatDateTime(item.createdAt),
      }));
      this.setData({ requests });
    } catch (e) {
      // handled
    } finally {
      this.setData({ loading: false });
    }
  },

  openCreate() {
    this.setData({
      showCreate: true,
      form: {
        type: 'leave',
        fromDate: formatDate(new Date()),
        targetUserId: '',
        toShiftId: '',
        reason: '',
      },
      targetIndex: 0,
      shiftIndex: 0,
    });
  },

  closeCreate() {
    this.setData({ showCreate: false });
  },

  noop() {},

  onTypeChange(e) {
    this.setData({ 'form.type': e.detail.value });
  },

  onDateChange(e) {
    this.setData({ 'form.fromDate': e.detail.value });
  },

  onTargetChange(e) {
    const index = Number(e.detail.value || 0);
    const target = this.data.staffList[index];
    this.setData({
      targetIndex: index,
      'form.targetUserId': target ? target.id : '',
    });
  },

  onShiftChange(e) {
    const index = Number(e.detail.value || 0);
    const shift = this.data.shifts[index];
    this.setData({
      shiftIndex: index,
      'form.toShiftId': shift ? shift.id : '',
    });
  },

  onReason(e) {
    this.setData({ 'form.reason': e.detail.value });
  },

  async onCreate() {
    const { form, staffList, shifts, targetIndex, shiftIndex, submitting } = this.data;
    if (submitting) return;
    if (!form.fromDate) {
      wx.showToast({ title: '请选择日期', icon: 'none' });
      return;
    }

    const payload = {
      type: form.type,
      fromDate: form.fromDate,
      reason: form.reason || undefined,
    };

    if (form.type === 'swap') {
      const target = staffList[targetIndex];
      if (!target) {
        wx.showToast({ title: '请选择调班对象', icon: 'none' });
        return;
      }
      payload.targetUserId = target.id;
    }

    if (form.type === 'change') {
      const shift = shifts[shiftIndex];
      if (!shift) {
        wx.showToast({ title: '请选择目标班次', icon: 'none' });
        return;
      }
      payload.toShiftId = shift.id;
    }

    this.setData({ submitting: true });
    try {
      await api.createRequest(payload);
      wx.showToast({ title: '已提交', icon: 'success' });
      this.setData({ showCreate: false });
      this.loadRequests();
    } catch (e) {
      // handled
    } finally {
      this.setData({ submitting: false });
    }
  },

  onApprove(e) {
    const id = e.currentTarget.dataset.id;
    wx.showModal({
      title: '审批确认',
      content: '确认通过该申请？通过后将自动更新排班表。',
      success: async (res) => {
        if (!res.confirm) return;
        try {
          await api.approveRequest(id);
          wx.showToast({ title: '已通过', icon: 'success' });
          this.loadRequests();
        } catch (err) {
          // handled
        }
      },
    });
  },

  onReject(e) {
    const id = e.currentTarget.dataset.id;
    wx.showModal({
      title: '驳回申请',
      editable: true,
      placeholderText: '可填写驳回原因',
      success: async (res) => {
        if (!res.confirm) return;
        try {
          await api.rejectRequest(id, res.content || undefined);
          wx.showToast({ title: '已驳回', icon: 'success' });
          this.loadRequests();
        } catch (err) {
          // handled
        }
      },
    });
  },
});

