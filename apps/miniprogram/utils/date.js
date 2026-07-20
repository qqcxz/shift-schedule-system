function pad(n) {
  return n < 10 ? `0${n}` : `${n}`;
}

function formatDate(date) {
  const d = date instanceof Date ? date : new Date(date);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function formatMonth(date) {
  const d = date instanceof Date ? date : new Date(date);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
}

function formatDateTime(value) {
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value || '';
  return `${formatDate(d)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function daysInMonth(month) {
  const [y, m] = month.split('-').map(Number);
  const total = new Date(y, m, 0).getDate();
  const days = [];
  for (let i = 1; i <= total; i += 1) {
    days.push(`${month}-${pad(i)}`);
  }
  return days;
}

function weekdayLabel(day) {
  const map = ['日', '一', '二', '三', '四', '五', '六'];
  return map[new Date(day).getDay()];
}

function isWeekend(day) {
  const w = new Date(day).getDay();
  return w === 0 || w === 6;
}

function shiftMonth(month, offset) {
  const [y, m] = month.split('-').map(Number);
  const d = new Date(y, m - 1 + offset, 1);
  return formatMonth(d);
}

function typeLabel(type) {
  if (type === 'leave') return '调休';
  if (type === 'swap') return '调班';
  if (type === 'change') return '改班';
  return type || '-';
}

function statusLabel(status) {
  if (status === 'pending') return '待审批';
  if (status === 'approved') return '已通过';
  if (status === 'rejected') return '已驳回';
  if (status === 'cancelled') return '已取消';
  return status || '-';
}

function statusClass(status) {
  if (status === 'pending') return 'tag-warning';
  if (status === 'approved') return 'tag-success';
  if (status === 'rejected') return 'tag-danger';
  return 'tag-info';
}

module.exports = {
  formatDate,
  formatMonth,
  formatDateTime,
  daysInMonth,
  weekdayLabel,
  isWeekend,
  shiftMonth,
  typeLabel,
  statusLabel,
  statusClass,
};
