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

function users() {
  return request({ url: '/users' });
}

function shifts() {
  return request({ url: '/shifts' });
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
  shifts,
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
