<template>
  <div class="page-card schedule-page">
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
        <el-button v-if="auth.isManager" @click="openCleanup">清理历史</el-button>
        <el-button v-if="auth.isManager" type="primary" :loading="saving" @click="saveAll">保存本月排班</el-button>
      </div>
    </div>

    <div v-if="auth.isManager && storage" class="storage-bar">
      <span>排班记录：{{ storage.total }} 条</span>
      <span v-if="storage.oldestDate">最早：{{ storage.oldestDate }}</span>
      <span v-if="storage.newestDate">最新：{{ storage.newestDate }}</span>
      <span class="muted">建议保留近 {{ storage.keepMonthsDefault }} 个月，定期清理更早数据</span>
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

    <el-dialog v-model="cleanupVisible" title="清理历史排班" class="responsive-dialog" width="480px">
      <div class="cleanup-tip muted">
        历史排班会持续增长。可只保留最近几个月，或删除指定月份之前的数据。删除后不可恢复。
      </div>
      <el-form class="mobile-form" label-width="120px" style="margin-top: 16px">
        <el-form-item label="清理方式">
          <el-radio-group v-model="cleanupMode">
            <el-radio-button value="keep">保留最近 N 个月</el-radio-button>
            <el-radio-button value="before">删除指定月份前</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="cleanupMode === 'keep'" label="保留月数">
          <el-input-number v-model="keepMonths" :min="1" :max="120" />
          <div class="muted tiny">含当前月。例如填 6，只保留最近 6 个月。</div>
        </el-form-item>
        <el-form-item v-else label="截止日期">
          <el-date-picker
            v-model="beforeMonth"
            type="month"
            value-format="YYYY-MM"
            format="YYYY年MM月"
            placeholder="删除该月之前的数据"
          />
          <div class="muted tiny">例如选 2026-01，会删除 2025-12 及更早的排班。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cleanupVisible = false">取消</el-button>
        <el-button type="danger" :loading="cleaning" @click="onCleanup">确认清理</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  cleanupSchedulesApi,
  saveMonthSchedulesApi,
  schedulesApi,
  scheduleStorageApi,
  shiftsApi,
  usersApi,
  type ScheduleItem,
  type ScheduleStorageSummary,
  type ShiftTemplate,
  type User,
} from '../api';
import { useAuthStore } from '../stores/auth';
import { onRealtime } from '../realtime';

const auth = useAuthStore();
const loading = ref(false);
const saving = ref(false);
const cleaning = ref(false);
const cleanupVisible = ref(false);
const cleanupMode = ref<'keep' | 'before'>('keep');
const keepMonths = ref(12);
const beforeMonth = ref(dayjs().subtract(12, 'month').format('YYYY-MM'));
const monthValue = ref(dayjs().format('YYYY-MM'));
const users = ref<User[]>([]);
const shifts = ref<ShiftTemplate[]>([]);
const items = ref<ScheduleItem[]>([]);
const storage = ref<ScheduleStorageSummary | null>(null);
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

async function loadStorage() {
  if (!auth.isManager) {
    storage.value = null;
    return;
  }
  try {
    const { data } = await scheduleStorageApi();
    storage.value = data;
    if (data?.keepMonthsDefault) {
      keepMonths.value = data.keepMonthsDefault;
    }
  } catch {
    storage.value = null;
  }
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
    await loadStorage();
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
    await loadStorage();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

function openCleanup() {
  cleanupMode.value = 'keep';
  keepMonths.value = storage.value?.keepMonthsDefault || 12;
  beforeMonth.value = dayjs().subtract(12, 'month').format('YYYY-MM');
  cleanupVisible.value = true;
}

async function onCleanup() {
  const payload =
    cleanupMode.value === 'keep'
      ? { keepMonths: keepMonths.value }
      : { beforeMonth: beforeMonth.value };

  if (cleanupMode.value === 'before' && !beforeMonth.value) {
    ElMessage.warning('请选择截止日期');
    return;
  }

  const tip =
    cleanupMode.value === 'keep'
      ? `确认只保留最近 ${keepMonths.value} 个月的排班？更早数据将永久删除。`
      : `确认删除 ${beforeMonth.value} 之前的全部排班？删除后不可恢复。`;

  try {
    await ElMessageBox.confirm(tip, '清理历史排班', { type: 'warning' });
  } catch {
    return;
  }

  cleaning.value = true;
  try {
    const { data } = await cleanupSchedulesApi(payload);
    storage.value = data.summary;
    ElMessage.success(`已清理 ${data.deleted} 条历史排班`);
    cleanupVisible.value = false;
    await loadAll();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    cleaning.value = false;
  }
}

onMounted(async () => {
  await loadAll();
  offRealtime = [
    onRealtime('schedule.updated', (payload: { month?: string }) => {
      if (!payload?.month || payload.month === monthValue.value) {
        loadAll();
      } else {
        loadStorage();
      }
    }),
    onRealtime('shifts.updated', () => {
      loadAll();
    }),
    onRealtime('users.updated', () => {
      loadAll();
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

.storage-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 18px;
  align-items: center;
  margin-bottom: 14px;
  padding: 10px 14px;
  border-radius: 12px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  font-size: 13px;
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

.cleanup-tip {
  line-height: 1.6;
}

.tiny {
  margin-top: 6px;
  font-size: 12px;
}

@media (max-width: 768px) {
  .storage-bar {
    flex-direction: column;
    align-items: flex-start;
  }

  .schedule-table th,
  .schedule-table td {
    min-width: 56px;
    padding: 6px 4px;
    font-size: 12px;
  }

  .sticky-col {
    min-width: 84px !important;
  }

  .shift-chip {
    min-width: 48px;
    padding: 3px 6px;
    font-size: 11px;
  }
}
</style>

