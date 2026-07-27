<template>
  <div class="page-card">
    <div class="toolbar">
      <div>
        <h3 style="margin: 0">消息中心</h3>
        <p class="muted" style="margin: 6px 0 0">申请结果、排班变更通知会在这里汇总</p>
      </div>
      <div class="actions">
        <el-button @click="load" :loading="loading">刷新</el-button>
        <el-button type="primary" @click="markAll">全部已读</el-button>
        <el-button @click="clearRead" :loading="clearing">清理已读</el-button>
        <el-button type="danger" plain @click="clearAll" :loading="clearing">清空全部</el-button>
      </div>
    </div>

    <el-empty v-if="!loading && !items.length" description="暂无消息" />
    <div v-else class="list">
      <div v-for="item in items" :key="item.id" class="item" :class="{ unread: !item.isRead }">
        <div class="item-main" @click="markOne(item)">
          <div class="item-title">
            <span>{{ item.title }}</span>
            <el-tag v-if="!item.isRead" size="small" type="danger">未读</el-tag>
          </div>
          <div class="item-content">{{ item.content }}</div>
          <div class="muted">{{ formatTime(item.createdAt) }}</div>
        </div>
        <div class="item-actions">
          <el-button link type="danger" @click.stop="removeOne(item)">删除</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  clearNotificationsApi,
  deleteNotificationApi,
  markAllNotificationsReadApi,
  markNotificationReadApi,
  notificationsApi,
  type NotificationItem,
} from '../api';
import { onRealtime } from '../realtime';

const loading = ref(false);
const clearing = ref(false);
const items = ref<NotificationItem[]>([]);
let offRealtime: Array<() => void> = [];

function formatTime(value: string) {
  return dayjs(value).format('YYYY-MM-DD HH:mm');
}

async function load() {
  loading.value = true;
  try {
    const { data } = await notificationsApi();
    items.value = data;
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function markOne(item: NotificationItem) {
  if (item.isRead) return;
  try {
    await markNotificationReadApi(item.id);
    item.isRead = true;
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function markAll() {
  try {
    await markAllNotificationsReadApi();
    items.value = items.value.map((item) => ({ ...item, isRead: true }));
    ElMessage.success('已全部标记为已读');
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function removeOne(item: NotificationItem) {
  try {
    await deleteNotificationApi(item.id);
    items.value = items.value.filter((row) => row.id !== item.id);
    ElMessage.success('已删除');
  } catch (error) {
    ElMessage.error((error as Error).message);
  }
}

async function clearRead() {
  if (!items.value.some((item) => item.isRead)) {
    ElMessage.info('当前没有已读消息');
    return;
  }

  try {
    await ElMessageBox.confirm('确认清理全部已读消息？', '清理消息', { type: 'warning' });
  } catch {
    return;
  }

  clearing.value = true;
  try {
    const { data } = await clearNotificationsApi('read');
    items.value = items.value.filter((item) => !item.isRead);
    ElMessage.success(`已清理 ${data.deleted} 条已读消息`);
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    clearing.value = false;
  }
}

async function clearAll() {
  if (!items.value.length) {
    ElMessage.info('当前没有消息');
    return;
  }

  try {
    await ElMessageBox.confirm('确认清空全部消息？此操作不可恢复。', '清空消息', { type: 'warning' });
  } catch {
    return;
  }

  clearing.value = true;
  try {
    const { data } = await clearNotificationsApi('all');
    items.value = [];
    ElMessage.success(`已清空 ${data.deleted} 条消息`);
  } catch (error) {
    ElMessage.error((error as Error).message);
  } finally {
    clearing.value = false;
  }
}

onMounted(async () => {
  await load();
  offRealtime = [
    onRealtime('notification.created', () => load()),
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

.list {
  display: grid;
  gap: 12px;
}

.item {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 14px 16px;
  background: #fff;
}

.item.unread {
  border-color: #fca5a5;
  background: #fff7f7;
}

.item-main {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

.item-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-weight: 700;
  margin-bottom: 6px;
}

.item-content {
  margin-bottom: 8px;
  line-height: 1.5;
  word-break: break-word;
}

.item-actions {
  flex-shrink: 0;
}

@media (max-width: 640px) {
  .item {
    flex-direction: column;
    gap: 8px;
  }

  .item-actions {
    align-self: flex-end;
  }
}
</style>
