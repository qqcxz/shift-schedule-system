import http from './http';

export type User = {
  id: string;
  username: string;
  displayName: string;
  role: 'manager' | 'staff';
  storeId: string;
  isActive?: boolean;
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
