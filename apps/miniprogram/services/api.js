const { request } = require('../utils/request');

function login(username, password) {
  return request({
    url: '/auth/login',
    method: 'POST',
    data: { username, password },
    auth: false,
  });
}

function me() {
  return request({ url: '/auth/me' });
}

function users(includeInactive) {
  return request({
    url: '/users',
    params: includeInactive ? { includeInactive: '1' } : undefined,
  });
}

function createUser(payload) {
  return request({ url: '/users', method: 'POST', data: payload });
}

function updateUser(id, payload) {
  return request({ url: `/users/${id}`, method: 'PATCH', data: payload });
}

function deleteUser(id) {
  return request({ url: `/users/${id}`, method: 'DELETE' });
}

function shifts() {
  return request({ url: '/shifts' });
}

function createShift(payload) {
  return request({ url: '/shifts', method: 'POST', data: payload });
}

function updateShift(id, payload) {
  return request({ url: `/shifts/${id}`, method: 'PATCH', data: payload });
}

function deleteShift(id) {
  return request({ url: `/shifts/${id}`, method: 'DELETE' });
}

function store() {
  return request({ url: '/stores/current' });
}

function schedules(month) {
  return request({ url: '/schedules', params: { month } });
}

function saveMonthSchedules(payload) {
  return request({ url: '/schedules/month', method: 'PUT', data: payload });
}

function requests(status) {
  return request({ url: '/requests', params: status ? { status } : {} });
}

function createRequest(payload) {
  return request({ url: '/requests', method: 'POST', data: payload });
}

function approveRequest(id, reviewNote) {
  return request({
    url: `/requests/${id}/approve`,
    method: 'POST',
    data: { reviewNote },
  });
}

function rejectRequest(id, reviewNote) {
  return request({
    url: `/requests/${id}/reject`,
    method: 'POST',
    data: { reviewNote },
  });
}

function notifications() {
  return request({ url: '/notifications' });
}

function markNotificationRead(id) {
  return request({ url: `/notifications/${id}/read`, method: 'POST' });
}

function markAllNotificationsRead() {
  return request({ url: '/notifications/read-all', method: 'POST' });
}

module.exports = {
  login,
  me,
  users,
  createUser,
  updateUser,
  deleteUser,
  shifts,
  createShift,
  updateShift,
  deleteShift,
  store,
  schedules,
  saveMonthSchedules,
  requests,
  createRequest,
  approveRequest,
  rejectRequest,
  notifications,
  markNotificationRead,
  markAllNotificationsRead,
};
