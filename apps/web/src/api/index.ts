import http from './http';

export type User = {
  id: string;
  username: string;
  displayName: string;
  role: 'manager' | 'staff';
  storeId: string;
  isActive?: boolean;
  mustChangePassword?: boolean;
};

export type ShiftTemplate = {
  id: string;
  name: string;
  code: string;
  startTime: string;
  endTime: string;
  color: string;
  sortOrder?: number;
  isOff: boolean;
};

export type ScheduleItem = {
  id: string;
  userId: string;
  userName: string;
  workDate: string;
  shiftTemplateId: string;
  shiftName: string;
  shiftColor: string;
  status: string;
  version: number;
  note?: string;
};

export type ScheduleRequest = {
  id: string;
  type: 'leave' | 'swap' | 'change';
  status: 'pending' | 'approved' | 'rejected' | 'cancelled';
  requesterId: string;
  requester?: User;
  targetUserId?: string;
  targetUser?: User | null;
  fromDate: string;
  toDate?: string;
  fromShiftId?: string;
  toShiftId?: string;
  reason?: string;
  reviewNote?: string;
  createdAt: string;
};

export type NotificationItem = {
  id: string;
  title: string;
  content: string;
  isRead: boolean;
  createdAt: string;
};

export type PushBinding = {
  provider: string;
  tokenBound: boolean;
  maskedToken?: string;
};

export type StorePushSettings = {
  schedulePublishEnabled: boolean;
  swapRequestEnabled: boolean;
  preShiftReminderEnabled: boolean;
  preShiftReminderTime: string;
  timezone: string;
};

export type PushDelivery = {
  id: string;
  userId: string;
  eventType: 'schedule_publish' | 'schedule_request' | 'swap_request' | 'pre_shift_reminder' | 'test';
  relatedId?: string;
  title: string;
  content: string;
  msgId?: string;
  state: 'pending' | 'preparing' | 'sending' | 'sent' | 'failed' | 'rate_limited';
  providerStatus?: string;
  providerResponse?: string;
  attempts: number;
  attemptedAt?: string;
  statusCheckedAt?: string;
  createdAt: string;
};

export const loginApi = (username: string, password: string) =>
  http.post<{ accessToken: string; user: User }>('/auth/login', { username, password });

export const meApi = () => http.get<User>('/auth/me');
export const changePasswordApi = (payload: {
  currentPassword: string;
  newPassword: string;
}) => http.post<{ ok: boolean }>('/auth/change-password', payload);
export const usersApi = (includeInactive = false) =>
  http.get<User[]>('/users', {
    params: includeInactive ? { includeInactive: '1' } : undefined,
  });
export const createUserApi = (payload: {
  username: string;
  displayName: string;
  password: string;
  role?: 'manager' | 'staff';
}) => http.post<User>('/users', payload);
export const updateUserApi = (
  id: string,
  payload: {
    username?: string;
    displayName?: string;
    password?: string;
    role?: 'manager' | 'staff';
    isActive?: boolean;
  },
) => http.patch<User>(`/users/${id}`, payload);
export const deleteUserApi = (id: string) => http.delete(`/users/${id}`);

export const shiftsApi = () => http.get<ShiftTemplate[]>('/shifts');
export const createShiftApi = (payload: {
  name: string;
  code?: string;
  startTime: string;
  endTime: string;
  color?: string;
  sortOrder?: number;
  isOff?: boolean;
}) => http.post<ShiftTemplate>('/shifts', payload);
export const updateShiftApi = (
  id: string,
  payload: {
    name?: string;
    code?: string;
    startTime?: string;
    endTime?: string;
    color?: string;
    sortOrder?: number;
    isOff?: boolean;
  },
) => http.patch<ShiftTemplate>(`/shifts/${id}`, payload);
export const deleteShiftApi = (id: string) => http.delete(`/shifts/${id}`);

export const storeApi = () => http.get<{ id: string; name: string; address?: string }>('/stores/current');

export const myPushBindingApi = () => http.get<PushBinding>('/push-settings/me');
export const saveMyPushBindingApi = (xiatuiToken: string) =>
  http.put<PushBinding>('/push-settings/me', { xiatuiToken });
export const deleteMyPushBindingApi = () => http.delete<{ ok: boolean }>('/push-settings/me/token');
export const testMyPushBindingApi = () => http.post<PushDelivery>('/push-settings/me/test');
export const storePushSettingsApi = () =>
  http.get<StorePushSettings>('/stores/current/push-settings');
export const updateStorePushSettingsApi = (payload: Partial<StorePushSettings>) =>
  http.patch<StorePushSettings>('/stores/current/push-settings', payload);
export const pushDeliveriesApi = () => http.get<PushDelivery[]>('/push-deliveries');
export const clearPushDeliveriesApi = () => http.delete<{ ok: boolean; deleted: number }>('/push-deliveries');
export const queryPushDeliveryStatusApi = (id: string) =>
  http.get<PushDelivery>(`/push-deliveries/${id}/status`);

export const schedulesApi = (month: string) =>
  http.get<{ month: string; items: ScheduleItem[] }>('/schedules', { params: { month } });
export const exportMonthSchedulesApi = (month: string) =>
  http.get<Blob>('/schedules/export', { params: { month }, responseType: 'blob' });
export const saveMonthSchedulesApi = (payload: {
  month: string;
  items: Array<{
    userId: string;
    workDate: string;
    shiftTemplateId: string;
    status?: string;
    note?: string;
    version?: number;
  }>;
}) => http.put('/schedules/month', payload);

export const publishMonthSchedulesApi = (month: string, message?: string) =>
  http.post<{ ok: boolean; month: string; queued: number; message?: string }>(
    `/schedules/${month}/publish`,
    message ? { message } : {},
  );

export type ScheduleStorageSummary = {
  total: number;
  oldestDate: string | null;
  newestDate: string | null;
  keepMonthsDefault: number;
};

export const scheduleStorageApi = () =>
  http.get<ScheduleStorageSummary>('/schedules/storage');

export const cleanupSchedulesApi = (payload: {
  beforeMonth?: string;
  keepMonths?: number;
}) =>
  http.post<{
    ok: boolean;
    beforeMonth: string;
    deleted: number;
    summary: ScheduleStorageSummary;
  }>('/schedules/cleanup', payload);

export const requestsApi = (status?: string) =>
  http.get<ScheduleRequest[]>('/requests', { params: status ? { status } : {} });

export const createRequestApi = (payload: {
  type: 'leave' | 'swap' | 'change';
  fromDate: string;
  toDate?: string;
  targetUserId?: string;
  fromShiftId?: string;
  toShiftId?: string;
  reason?: string;
}) => http.post<ScheduleRequest>('/requests', payload);

export const approveRequestApi = (id: string, reviewNote?: string) =>
  http.post(`/requests/${id}/approve`, { reviewNote });

export const rejectRequestApi = (id: string, reviewNote?: string) =>
  http.post(`/requests/${id}/reject`, { reviewNote });

export const clearResolvedRequestsApi = () =>
  http.post<{ ok: boolean; deleted: number }>('/requests/clear-resolved');

export const deleteRequestApi = (id: string) =>
  http.delete<{ ok: boolean; deleted: number }>(`/requests/${id}`);

export const notificationsApi = () => http.get<NotificationItem[]>('/notifications');
export const markNotificationReadApi = (id: string) => http.post(`/notifications/${id}/read`);
export const markAllNotificationsReadApi = () => http.post('/notifications/read-all');
export const clearNotificationsApi = (mode: 'read' | 'all' = 'read') =>
  http.post<{ ok: boolean; deleted: number; mode: string }>('/notifications/clear', { mode });
export const deleteNotificationApi = (id: string) =>
  http.delete<{ ok: boolean; deleted: number }>(`/notifications/${id}`);
