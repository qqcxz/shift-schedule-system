<template>
  <div class="page-card">
    <div class="toolbar">
      <div class="left">
        <el-date-picker
          v-model="monthValue"
          type="month"
          value-format="YYYY-MM"
          format="YYYY年MM月"
          @change="loadAll"
        />
        <el-tag v-if="auth.isManager" type="info">点击单元格可切换班次，改完后点保存</el-tag>
        <el-tag v-else type="success">可查看全店排班，并在“申请审批”中提交调休/调班</el-tag>
      </div>
      <div class="right">
        <el-button @click="loadAll" :loading="loading">刷新</el-button>
        <el-button v-if="auth.isManager" type="primary" :loading="saving" @click="saveAll">保存本月排班</el-button>
      </div>
    </div>

    <div class="legend">
      <span v-for="shift in shifts" :key="shift.id" class="legend-item">
        <span class="shift-chip" :style="{ background: shift.color }">{{ shift.name }}</span>
        <span class="muted">{{ shift.isOff ? '休息' : `${shift.startTime}-${shift.endTime}` }}</span>
      </span>
    </div>

    <div class="table-wrap">
      <table class="schedule-table">
        <thead>
          <tr>
            <th class="sticky-col">员工</th>
            <th v-for="day in days" :key="day" :class="{ weekend: isWeekend(day) }">
              <div>{{ day.slice(-2) }}</div>
              <div class="weekday">{{ weekdayLabel(day) }}</div>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in staffUsers" :key="user.id">
            <td class="sticky-col user-cell">
              <div class="user-name">{{ user.displayName }}</div>
              <div class="muted">{{ user.role === 'manager' ? '店长' : '店员' }}</div>
            </td>
            <td
              v-for="day in days"
              :key="`${user.id}-${day}`"
              :class="{ weekend: isWeekend(day), editable: auth.isManager }"
              @click="cycleShift(user.id, day)"
            >
              <span
                v-if="cellMap[`${user.id}_${day}`]"
                class="shift-chip"
                :style="{ background: cellMap[`${user.id}_${day}`].shiftColor }"
              >
                {{ cellMap[`${user.id}_${day}`].shiftName }}
              </span>
              <span v-else class="muted">-</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage } from 'element-plus';
import {
  saveMonthSchedulesApi,
  schedulesApi,
  shiftsApi,
  usersApi,
  type ScheduleItem,
  type ShiftTemplate,
  type User,
} from '../api';
import { useAuthStore } from '../stores/auth';
import { onRealtime } from '../realtime';

const auth = useAuthStore();
const loading = ref(false);
const saving = ref(false);
const monthValue = ref(dayjs().format('YYYY-MM'));
const users = ref<User[]>([]);
const shifts = ref<ShiftTemplate[]>([]);
const items = ref<ScheduleItem[]>([]);
const drafts = reactive<Record<string, { shiftTemplateId: string; version?: number; status?: string }>>({});
let offRealtime: Array<() => void> = [];

const staffUsers = computed(() => users.value.filter((item) => item.role === 'staff' || item.role === 'manager'));

const days = computed(() => {
  const start = dayjs(`${monthValue.value}-01`);
  const total = start.daysInMonth();
  return Array.from({ length: total }, (_, index) => start.date(index + 1).format('YYYY-MM-DD'));
});

const cellMap = computed(() => {
  const map: Record<string, ScheduleItem> = {};
  for (const item of items.value) {
    const key = `${item.userId}_${item.workDate}`;
    const draft = drafts[key];
    if (draft) {
      const shift = shifts.value.find((s) => s.id === draft.shiftTemplateId);
      map[key] = {
        ...item,
        shiftTemplateId: draft.shiftTemplateId,
        shiftName: shift?.name || item.shiftName,
        shiftColor: shift?.color || item.shiftColor,
        version: draft.version ?? item.version,
        status: draft.status || item.status,
      };
    } else {
      map[key] = item;
    }
  }

  // include pure drafts for empty cells
  for (const [key, draft] of Object.entries(drafts)) {
    if (map[key]) continue;
    const [userId, workDate] = key.split('_');
    const shift = shifts.value.find((s) => s.id === draft.shiftTemplateId);
    map[key] = {
      id: '',
      userId,
      userName: '',
      workDate,
      shiftTemplateId: draft.shiftTemplateId,
      shiftName: shift?.name || '-',
      shiftColor: shift?.color || '#909399',
      status: draft.status || 'normal',
      version: draft.version || 1,
    };
  }
  return map;
});

function weekdayLabel(day: string) {
  return ['日', '一', '二', '三', '四', '五', '六'][dayjs(day).day()];
}

function isWeekend(day: string) {
  const d = dayjs(day).day();
  return d === 0 || d === 6;
}

function cycleShift(userId: string, day: string) {
  if (!auth.isManager || !shifts.value.length) return;
  const key = `${userId}_${day}`;
  const current = cellMap.value[key];
  const currentIndex = current
    ? shifts.value.findIndex((item) => item.id === current.shiftTemplateId)
    : -1;
  const next = shifts.value[(currentIndex + 1) % shifts.value.length];
  drafts[key] = {
    shiftTemplateId: next.id,
    version: current?.version,
    status: next.isOff ? 'leave' : 'normal',
  };
}

async function loadAll() {
  loading.value = true;
  try {
    const [usersRes, shiftsRes, schedulesRes] = await Promise.all([
      usersApi(),
      shiftsApi(),
      schedulesApi(monthValue.value),
    ]);
    users.value = usersRes.data;
    shifts.value = shiftsRes.data;
    items.value = schedulesRes.data.items;
    Object.keys(drafts).forEach((key) => delete drafts[key]);
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function saveAll() {
  const payloadItems = Object.entries(drafts).map(([key, draft]) => {
    const [userId, workDate] = key.split('_');
    return {
      userId,
      workDate,
      shiftTemplateId: draft.shiftTemplateId,
      status: draft.status,
      version: draft.version,
    };
  });

  if (!payloadItems.length) {
    ElMessage.info('没有需要保存的修改');
    return;
  }

  saving.value = true;
  try {
    const { data } = await saveMonthSchedulesApi({
      month: monthValue.value,
      items: payloadItems,
    });
    items.value = data.items;
    Object.keys(drafts).forEach((key) => delete drafts[key]);
    ElMessage.success('排班已保存，并已实时同步');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await loadAll();
  offRealtime = [
    onRealtime('schedule.updated', (payload: { month?: string }) => {
      if (!payload?.month || payload.month === monthValue.value) {
        loadAll();
      }
    }),
  ];
});

onUnmounted(() => {
  offRealtime.forEach((off) => off());
});
</script>

<style scoped>
.left,
.right {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 14px;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.table-wrap {
  overflow: auto;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
}

.schedule-table {
  border-collapse: separate;
  border-spacing: 0;
  min-width: 100%;
  background: #fff;
}

.schedule-table th,
.schedule-table td {
  border-right: 1px solid #eef2f7;
  border-bottom: 1px solid #eef2f7;
  padding: 8px;
  text-align: center;
  min-width: 72px;
  white-space: nowrap;
}

.schedule-table th {
  position: sticky;
  top: 0;
  background: #f8fafc;
  z-index: 2;
}

.sticky-col {
  position: sticky;
  left: 0;
  background: #fff;
  z-index: 3;
  min-width: 110px !important;
}

.schedule-table th.sticky-col {
  z-index: 4;
  background: #f8fafc;
}

.weekend {
  background: #fcfcfd;
}

.editable {
  cursor: pointer;
}

.editable:hover {
  background: #eff6ff;
}

.user-cell {
  text-align: left;
}

.user-name {
  font-weight: 600;
}

.weekday {
  color: #9ca3af;
  font-size: 12px;
  font-weight: 400;
}
</style>
