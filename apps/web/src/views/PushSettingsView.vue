<template>
  <div class="settings-grid">
    <el-card class="page-card">
      <template #header>
        <div class="card-heading">
          <div>
            <div class="heading">我的虾推啥 Token</div>
            <div class="muted">店长和店员都可以绑定自己的 Token，仅用于向本人发送微信通知。</div>
          </div>
          <el-tag :type="binding?.tokenBound ? 'success' : 'info'">
            {{ binding?.tokenBound ? '已绑定' : '未绑定' }}
          </el-tag>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon>
        请先关注“虾推啥”公众号，复制公众号发送给你的 Token。Token 只会保存为加密凭证，页面不会回显完整内容。
      </el-alert>
      <div v-if="binding?.tokenBound" class="bound-token">
        当前 Token：<strong>{{ binding.maskedToken }}</strong>
      </div>
      <el-form label-position="top" class="settings-form" @submit.prevent="saveBinding">
        <el-form-item :label="binding?.tokenBound ? '换绑 Token' : '绑定 Token'">
          <el-input
            v-model="token"
            type="password"
            show-password
            autocomplete="new-password"
            placeholder="粘贴虾推啥 Token"
            maxlength="256"
          />
          <div class="muted tiny">Token 仅用于个人推送，不要把 Token 发给其他人。</div>
        </el-form-item>
        <div class="form-actions">
          <el-button type="primary" :loading="saving" @click="saveBinding">
            {{ binding?.tokenBound ? '换绑 Token' : '绑定 Token' }}
          </el-button>
          <el-button v-if="binding?.tokenBound" :loading="testing" @click="sendTest">发送测试消息</el-button>
          <el-button v-if="binding?.tokenBound" type="danger" plain :loading="removing" @click="removeBinding">解绑</el-button>
        </div>
      </el-form>
    </el-card>

    <el-card v-if="auth.isManager" class="page-card">
      <template #header>
        <div>
          <div class="heading">门店推送事件</div>
          <div class="muted">这些开关只控制是否发送微信通知，不会影响排班或申请业务本身。</div>
        </div>
      </template>
      <el-form label-position="top" class="settings-form">
        <el-form-item label="排班发布通知">
          <div class="switch-row">
            <span class="muted">发布排班后，向本店已绑定 Token 的员工发送排班摘要。</span>
            <el-switch v-model="storeSettings.schedulePublishEnabled" @change="saveStoreSettings" />
          </div>
        </el-form-item>
        <el-form-item label="排班申请通知">
          <div class="switch-row">
            <span class="muted">员工提交调休、调班或改班申请后，通知已绑定 Token 的店长。</span>
            <el-switch v-model="storeSettings.swapRequestEnabled" @change="saveStoreSettings" />
          </div>
        </el-form-item>
        <el-form-item label="班前提醒">
          <div class="switch-row">
            <span class="muted">每天在设定时间提醒下一班；当天没有待开始班次时，自动提醒次日班次。</span>
            <el-switch v-model="storeSettings.preShiftReminderEnabled" @change="saveStoreSettings" />
          </div>
        </el-form-item>
        <el-form-item label="提醒时间">
          <el-time-picker
            v-model="storeSettings.preShiftReminderTime"
            format="HH:mm"
            value-format="HH:mm"
            placeholder="选择每天推送时间"
            :clearable="false"
            @change="saveStoreSettings"
          />
          <span class="muted minute-suffix">每天在该时间推送班前提醒</span>
        </el-form-item>
        <el-form-item label="门店时区">
          <el-select v-model="storeSettings.timezone" @change="saveStoreSettings">
            <el-option label="中国标准时间（Asia/Shanghai）" value="Asia/Shanghai" />
            <el-option label="香港时间（Asia/Hong_Kong）" value="Asia/Hong_Kong" />
            <el-option label="台北时间（Asia/Taipei）" value="Asia/Taipei" />
          </el-select>
        </el-form-item>
      </el-form>
      <div v-if="settingsSavedAt" class="muted tiny">设置已保存</div>
    </el-card>

    <el-card class="page-card history-card">
      <template #header>
        <div class="card-heading">
          <div>
            <div class="heading">最近推送记录</div>
            <div class="muted">店长可清空本店（含本人和员工）记录，店员仅可清空自己的记录。</div>
          </div>
          <div class="history-actions">
            <el-button text :loading="historyLoading" @click="loadHistory">刷新</el-button>
            <el-button text type="danger" :disabled="!deliveries.length" :loading="clearingHistory" @click="clearHistory">
              {{ auth.isManager ? '清空全店记录' : '清空我的记录' }}
            </el-button>
          </div>
        </div>
      </template>
      <el-empty v-if="!historyLoading && !deliveries.length" description="暂无推送记录" />
      <div v-else class="history-content" v-loading="historyLoading">
        <div class="desktop-history table-scroll">
          <el-table :data="deliveries" size="small" stripe>
            <el-table-column prop="title" label="消息" min-width="180" />
            <el-table-column prop="state" label="状态" width="110">
              <template #default="scope">
                <el-tag :type="stateType(scope.row.state)" size="small">{{ stateLabel(scope.row.state) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="providerStatus" label="服务商状态" min-width="130" />
            <el-table-column prop="msgId" label="msg_id" min-width="240" />
            <el-table-column prop="createdAt" label="创建时间" min-width="170" />
            <el-table-column label="操作" width="100">
              <template #default="scope">
                <el-button v-if="scope.row.msgId" link type="primary" @click="queryStatus(scope.row)">查状态</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="mobile-history-list">
          <article v-for="item in deliveries" :key="item.id" class="mobile-history-item">
            <div class="mobile-history-top">
              <div class="mobile-history-title">{{ item.title }}</div>
              <el-tag :type="stateType(item.state)" size="small">{{ stateLabel(item.state) }}</el-tag>
            </div>
            <div class="mobile-history-meta">{{ eventLabel(item.eventType) }} · {{ formatCreatedAt(item.createdAt) }}</div>
            <div v-if="item.providerStatus" class="mobile-history-detail">服务商：{{ item.providerStatus }}</div>
            <div v-if="item.msgId" class="mobile-history-msg-id">{{ item.msgId }}</div>
            <el-button v-if="item.msgId" link type="primary" class="mobile-history-action" @click="queryStatus(item)">
              查询投递状态
            </el-button>
          </article>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  clearPushDeliveriesApi,
  deleteMyPushBindingApi,
  myPushBindingApi,
  pushDeliveriesApi,
  queryPushDeliveryStatusApi,
  saveMyPushBindingApi,
  storePushSettingsApi,
  testMyPushBindingApi,
  updateStorePushSettingsApi,
  type PushBinding,
  type PushDelivery,
  type StorePushSettings,
} from '../api';
import { useAuthStore } from '../stores/auth';

const auth = useAuthStore();
const binding = ref<PushBinding | null>(null);
const token = ref('');
const saving = ref(false);
const testing = ref(false);
const removing = ref(false);
const historyLoading = ref(false);
const clearingHistory = ref(false);
const settingsSavedAt = ref(false);
const deliveries = ref<PushDelivery[]>([]);
const storeSettings = reactive<StorePushSettings>({
  schedulePublishEnabled: false,
  swapRequestEnabled: false,
  preShiftReminderEnabled: false,
  preShiftReminderTime: '08:00',
  timezone: 'Asia/Shanghai',
});

async function loadBinding() {
  const { data } = await myPushBindingApi();
  binding.value = data;
}

async function loadStoreSettings() {
  if (!auth.isManager) return;
  const { data } = await storePushSettingsApi();
  Object.assign(storeSettings, data);
}

async function loadHistory() {
  historyLoading.value = true;
  try {
    const { data } = await pushDeliveriesApi();
    deliveries.value = data;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    historyLoading.value = false;
  }
}

async function loadAll() {
  try {
    await Promise.all([loadBinding(), loadStoreSettings(), loadHistory()]);
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function saveBinding() {
  if (!token.value.trim()) {
    ElMessage.warning('请粘贴虾推啥 Token');
    return;
  }
  saving.value = true;
  try {
    const { data } = await saveMyPushBindingApi(token.value.trim());
    binding.value = data;
    token.value = '';
    ElMessage.success('Token 已安全保存');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

async function removeBinding() {
  try {
    await ElMessageBox.confirm('解绑后将不会再收到虾推啥微信通知，确定继续吗？', '确认解绑', {
      type: 'warning',
      confirmButtonText: '解绑',
      cancelButtonText: '取消',
    });
  } catch {
    return;
  }
  removing.value = true;
  try {
    await deleteMyPushBindingApi();
    binding.value = { provider: '虾推啥', tokenBound: false };
    ElMessage.success('Token 已解绑');
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    removing.value = false;
  }
}

async function sendTest() {
  testing.value = true;
  try {
    const { data } = await testMyPushBindingApi();
    deliveries.value = [data, ...deliveries.value.filter((item) => item.id !== data.id)];
    ElMessage.success(data.state === 'sent' ? '测试消息已发送' : `测试消息状态：${stateLabel(data.state)}`);
  } catch (error) {
    ElMessage.error((error as Error).message);
    await loadHistory();
  } finally {
    testing.value = false;
  }
}

let settingsTimer: number | undefined;
let settingsRequestVersion = 0;
async function saveStoreSettings() {
  window.clearTimeout(settingsTimer);
  const version = ++settingsRequestVersion;
  settingsTimer = window.setTimeout(async () => {
    try {
      const { data } = await updateStorePushSettingsApi({ ...storeSettings });
      if (version !== settingsRequestVersion) return;
      Object.assign(storeSettings, data);
      settingsSavedAt.value = true;
      window.setTimeout(() => (settingsSavedAt.value = false), 1800);
    } catch (error) {
      if (version === settingsRequestVersion) ElMessage.error((error as Error).message);
    }
  }, 200);
}

async function queryStatus(item: PushDelivery) {
  try {
    const { data } = await queryPushDeliveryStatusApi(item.id);
    const index = deliveries.value.findIndex((candidate) => candidate.id === item.id);
    if (index >= 0) deliveries.value[index] = data;
    ElMessage.info(`服务商状态：${data.providerStatus || '未知'}`);
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function clearHistory() {
  if (clearingHistory.value || !deliveries.value.length) return;
  try {
    await ElMessageBox.confirm(
      auth.isManager
        ? '将删除本店全部推送记录（包括店长和所有员工），且无法恢复。'
        : '将删除您自己的全部推送记录，且无法恢复。',
      '清空推送记录',
      { type: 'warning', confirmButtonText: '清空', cancelButtonText: '取消' },
    );
  } catch {
    return;
  }
  clearingHistory.value = true;
  try {
    const { data } = await clearPushDeliveriesApi();
    deliveries.value = [];
    ElMessage.success(`已清空 ${data.deleted} 条记录`);
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    clearingHistory.value = false;
  }
}

function stateLabel(state: PushDelivery['state']) {
  return { pending: '待发送', preparing: '准备发送', sending: '发送中', sent: '已发送', failed: '失败', rate_limited: '限流等待' }[state] || state;
}
function eventLabel(eventType: PushDelivery['eventType']) {
  return {
    schedule_publish: '排班发布',
    schedule_request: '排班申请',
    swap_request: '换班申请',
    pre_shift_reminder: '班前提醒',
    test: '测试消息',
  }[eventType] || eventType;
}
function formatCreatedAt(value: string) {
  return value ? value.replace('T', ' ').slice(0, 16) : '-';
}
function stateType(state: PushDelivery['state']) {
  return state === 'sent' ? 'success' : state === 'failed' ? 'danger' : state === 'rate_limited' ? 'warning' : 'info';
}

onMounted(loadAll);
</script>

<style scoped>
.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.page-card {
  min-width: 0;
}
.history-card {
  grid-column: 1 / -1;
}
.card-heading,
.switch-row,
.form-actions,
.history-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}
.card-heading > div:first-child {
  min-width: 0;
}
.history-actions {
  flex: 0 0 auto;
  justify-content: flex-end;
}
.history-content {
  min-width: 0;
}
.mobile-history-list {
  display: none;
}
.mobile-history-item {
  padding: 14px 0;
  border-top: 1px solid #eef0f3;
}
.mobile-history-item:first-child {
  border-top: none;
}
.mobile-history-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.mobile-history-title {
  min-width: 0;
  color: #1f2937;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}
.mobile-history-meta,
.mobile-history-detail {
  margin-top: 5px;
  color: #7b8492;
  font-size: 13px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}
.mobile-history-msg-id {
  margin-top: 10px;
  padding: 9px 10px;
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.4;
  overflow-wrap: anywhere;
}
.mobile-history-action {
  margin-top: 8px;
  padding: 0 !important;
}
.heading {
  font-size: 17px;
  font-weight: 700;
}
.bound-token {
  margin: 18px 0 8px;
  padding: 12px 14px;
  border-radius: 10px;
  background: #f0fdf4;
  color: #166534;
}
.settings-form {
  margin-top: 18px;
}
.switch-row .muted {
  flex: 1;
}
.minute-suffix {
  margin-left: 8px;
}
.tiny {
  margin-top: 6px;
  font-size: 12px;
}
@media (max-width: 900px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
  .history-card {
    grid-column: auto;
  }
  .switch-row {
    align-items: flex-start;
  }
}
@media (max-width: 768px) {
  .page-card :deep(.el-card__header) {
    padding: 14px 12px;
  }
  .page-card :deep(.el-card__body) {
    padding: 14px 12px;
  }
  .card-heading {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .history-actions {
    width: 100%;
    justify-content: flex-end;
  }
  .form-actions {
    flex-wrap: wrap;
  }
  .form-actions .el-button {
    margin-left: 0;
  }
  .switch-row {
    flex-wrap: wrap;
  }
  .switch-row .muted {
    min-width: 0;
  }
  .desktop-history {
    display: none;
  }
  .mobile-history-list {
    display: flex;
    flex-direction: column;
  }
}
</style>
