import http from './http';

export type User = {
  id: string;
  username: string;
  displayName: string;
  role: 'manager' | 'staff';
  storeId: string;
};

export type ShiftTemplate = {
  id: string;
  name: string;
  code: string;
  startTime: string;
  endTime: string;
  color: string;
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

export const loginApi = (username: string, password: string) =>
  http.post<{ accessToken: string; user: User }>('/auth/login', { username, password });

export const meApi = () => http.get<User>('/auth/me');
export const usersApi = () => http.get<User[]>('/users');
export const shiftsApi = () => http.get<ShiftTemplate[]>('/shifts');
export const storeApi = () => http.get<{ id: string; name: string; address?: string }>('/stores/current');
export const schedulesApi = (month: string) =>
  http.get<{ month: string; items: ScheduleItem[] }>('/schedules', { params: { month } });
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

export const notificationsApi = () => http.get<NotificationItem[]>('/notifications');
export const markNotificationReadApi = (id: string) => http.post(`/notifications/${id}/read`);
export const markAllNotificationsReadApi = () => http.post('/notifications/read-all');
