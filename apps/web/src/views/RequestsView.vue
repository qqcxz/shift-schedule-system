<template>
  <div class="grid">
    <div class="page-card">
      <div class="toolbar">
        <div>
          <h3 style="margin: 0">{{ auth.isManager ? '待处理 / 全部申请' : '我的申请' }}</h3>
          <p class="muted" style="margin: 6px 0 0">审批通过后排班会自动更新并实时推送</p>
        </div>
        <div class="actions">
          <el-select v-model="status" clearable placeholder="状态筛选" class="status-filter" @change="loadRequests">
            <el-option label="待审批" value="pending" />
            <el-option label="已通过" value="approved" />
            <el-option label="已驳回" value="rejected" />
          </el-select>
          <el-button @click="loadRequests" :loading="loading">刷新</el-button>
          <el-button @click="clearResolved" :loading="clearing">清理已处理</el-button>
          <el-button v-if="!auth.isManager" type="primary" @click="dialogVisible = true">新建申请</el-button>
        </div>
      </div>

      <div class="table-scroll desktop-only">
        <el-table :data="requests" stripe v-loading="loading">
          <el-table-column prop="type" label="类型" width="100">
            <template #default="{ row }">{{ typeLabel(row.type) }}</template>
          </el-table-column>
          <el-table-column label="申请人" min-width="120">
            <template #default="{ row }">{{ row.requester?.displayName || row.requesterId }}</template>
          </el-table-column>
          <el-table-column prop="fromDate" label="日期" width="120" />
          <el-table-column label="对象/目标" min-width="140">
            <template #default="{ row }">
              <span v-if="row.type === 'swap'">{{ row.targetUser?.displayName || row.targetUserId || '-' }}</span>
              <span v-else-if="row.type === 'change'">{{ shiftName(row.toShiftId) }}</span>
              <span v-else>休息</span>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="160" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <template v-if="auth.isManager && row.status === 'pending'">
                <el-button type="success" link @click="onApprove(row.id)">通过</el-button>
                <el-button type="danger" link @click="onReject(row.id)">驳回</el-button>
              </template>
              <el-button
                v-if="row.status !== 'pending'"
                type="danger"
                link
                @click="onDelete(row.id)"
              >
                删除
              </el-button>
              <span v-else-if="!auth.isManager" class="muted">待处理</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="mobile-list mobile-only" v-loading="loading">
        <el-empty v-if="!loading && !requests.length" description="暂无申请" />
        <div v-for="row in requests" :key="row.id" class="request-card">
          <div class="request-head">
            <div class="request-title">
              <strong>{{ typeLabel(row.type) }}</strong>
              <el-tag size="small" :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag>
            </div>
            <div class="muted">{{ row.fromDate }}</div>
          </div>
          <div class="request-body">
            <div>申请人：{{ row.requester?.displayName || row.requesterId }}</div>
            <div>
              对象/目标：
              <span v-if="row.type === 'swap'">{{ row.targetUser?.displayName || row.targetUserId || '-' }}</span>
              <span v-else-if="row.type === 'change'">{{ shiftName(row.toShiftId) }}</span>
              <span v-else>休息</span>
            </div>
            <div v-if="row.reason">原因：{{ row.reason }}</div>
          </div>
          <div class="request-actions">
            <template v-if="auth.isManager && row.status === 'pending'">
              <el-button type="success" size="small" @click="onApprove(row.id)">通过</el-button>
              <el-button type="danger" size="small" @click="onReject(row.id)">驳回</el-button>
            </template>
            <el-button
              v-if="row.status !== 'pending'"
              type="danger"
              plain
              size="small"
              @click="onDelete(row.id)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新建调休/调班申请"
      class="responsive-dialog"
      width="520px"
    >
      <el-form label-width="96px" class="mobile-form">
        <el-form-item label="申请类型">
          <el-radio-group v-model="form.type">
            <el-radio-button value="leave">调休</el-radio-button>
            <el-radio-button value="swap">调班</el-radio-button>
            <el-radio-button value="change">改班</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="form.fromDate" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
        </el-form-item>
        <el-form-item v-if="form.type === 'swap'" label="调班对象">
          <el-select v-model="form.targetUserId" placeholder="选择调班对象" style="width: 100%">
            <el-option
              v-for="user in swapTargets"
              :key="user.id"
              :label="`${user.displayName}（${user.role === 'manager' ? '店长' : '店员'}）`"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.type === 'change'" label="目标班次">
          <el-select v-model="form.toShiftId" placeholder="选择班次" style="width: 100%">
            <el-option
              v-for="shift in shifts"
              :key="shift.id"
              :label="`${shift.name} (${shift.isOff ? '休息' : shift.startTime + '-' + shift.endTime})`"
              :value="shift.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="原因">
          <el-input v-model="form.reason" type="textarea" :rows="3" placeholder="说明申请原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="onCreate">提交申请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  approveRequestApi,
  clearResolvedRequestsApi,
  createRequestApi,
  deleteRequestApi,
  rejectRequestApi,
  requestsApi,
  shiftsApi,
  usersApi,
  type ScheduleRequest,
  type ShiftTemplate,
  type User,
} from '../api';
import { useAuthStore } from '../stores/auth';
import { onRealtime } from '../realtime';

const auth = useAuthStore();
const loading = ref(false);
const submitting = ref(false);
const clearing = ref(false);
const dialogVisible = ref(false);
const status = ref<string | undefined>(auth.isManager ? 'pending' : undefined);
const requests = ref<ScheduleRequest[]>([]);
const users = ref<User[]>([]);
const shifts = ref<ShiftTemplate[]>([]);
let offRealtime: Array<() => void> = [];

const form = reactive({
  type: 'leave' as 'leave' | 'swap' | 'change',
  fromDate: dayjs().format('YYYY-MM-DD'),
  targetUserId: '',
  toShiftId: '',
  reason: '',
});

const swapTargets = computed(() =>
  users.value.filter((item) => item.isActive && item.id !== auth.user?.id),
);

function typeLabel(type: string) {
  if (type === 'leave') return '调休';
  if (type === 'swap') return '调班';
  return '改班';
}

function statusLabel(value: string) {
  if (value === 'pending') return '待审批';
  if (value === 'approved') return '已通过';
  if (value === 'rejected') return '已驳回';
  return value;
}

function statusType(value: string) {
  if (value === 'pending') return 'warning';
  if (value === 'approved') return 'success';
  if (value === 'rejected') return 'danger';
  return 'info';
}

function shiftName(id?: string) {
  return shifts.value.find((item) => item.id === id)?.name || id || '-';
}

async function loadRequests() {
  loading.value = true;
  try {
    const { data } = await requestsApi(status.value);
    requests.value = data;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function loadMeta() {
  const [usersRes, shiftsRes] = await Promise.all([usersApi(), shiftsApi()]);
  users.value = usersRes.data;
  shifts.value = shiftsRes.data;
}

async function onCreate() {
  if (!form.fromDate) {
    ElMessage.warning('请选择日期');
    return;
  }
  if (form.type === 'swap' && !form.targetUserId) {
    ElMessage.warning('请选择调班对象');
    return;
  }
  if (form.type === 'change' && !form.toShiftId) {
    ElMessage.warning('请选择目标班次');
    return;
  }

  submitting.value = true;
  try {
    await createRequestApi({
      type: form.type,
      fromDate: form.fromDate,
      targetUserId: form.type === 'swap' ? form.targetUserId : undefined,
      toShiftId: form.type === 'change' ? form.toShiftId : undefined,
      reason: form.reason,
    });
    dialogVisible.value = false;
    form.reason = '';
    ElMessage.success('申请已提交');
    await loadRequests();
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    submitting.value = false;
  }
}

async function onApprove(id: string) {
  try {
    await ElMessageBox.confirm('确认通过该申请？通过后将自动更新排班表。', '审批确认');
    await approveRequestApi(id);
    ElMessage.success('已通过并更新排班');
    await loadRequests();
  } catch (error) {
    if ((error as Error).message !== 'cancel') {
      ElMessage.error((error as Error).message);
    }
  }
}

async function onReject(id: string) {
  try {
    const { value } = await ElMessageBox.prompt('可填写驳回原因', '驳回申请', {
      inputPlaceholder: '可选',
      confirmButtonText: '驳回',
    });
    await rejectRequestApi(id, value);
    ElMessage.success('已驳回');
    await loadRequests();
  } catch (error) {
    if ((error as Error).message !== 'cancel') {
      ElMessage.error((error as Error).message);
    }
  }
}

async function onDelete(id: string) {
  try {
    await ElMessageBox.confirm('确认删除这条已处理申请？', '删除申请', { type: 'warning' });
    await deleteRequestApi(id);
    requests.value = requests.value.filter((item) => item.id !== id);
    ElMessage.success('已删除');
  } catch (error) {
    if ((error as Error).message !== 'cancel') {
      ElMessage.error((error as Error).message);
    }
  }
}

async function clearResolved() {
  try {
    await ElMessageBox.confirm(
      auth.isManager
        ? '确认清理全部已通过/已驳回的申请记录？待审批不会删除。'
        : '确认清理我的已通过/已驳回申请记录？',
      '清理申请记录',
      { type: 'warning' },
    );
  } catch {
    return;
  }

  clearing.value = true;
  try {
    const { data } = await clearResolvedRequestsApi();
    await loadRequests();
    ElMessage.success(`已清理 ${data.deleted} 条记录`);
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    clearing.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadMeta(), loadRequests()]);
  offRealtime = [
    onRealtime('request.created', () => loadRequests()),
    onRealtime('request.resolved', () => loadRequests()),
    onRealtime('request.cleared', () => loadRequests()),
  ];
});

onUnmounted(() => {
  offRealtime.forEach((off) => off());
});
</script>

<style scoped>
.actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.status-filter {
  width: 140px;
}

.table-scroll {
  width: 100%;
  overflow-x: auto;
}

.mobile-list {
  display: none;
  gap: 12px;
}

.request-card {
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 14px;
  background: #fff;
}

.request-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.request-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.request-body {
  display: grid;
  gap: 6px;
  color: #374151;
  line-height: 1.5;
  word-break: break-word;
}

.request-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.mobile-only {
  display: none;
}

@media (max-width: 768px) {
  .desktop-only {
    display: none;
  }

  .mobile-only {
    display: grid;
  }

  .status-filter {
    width: 100%;
  }
}
</style>
