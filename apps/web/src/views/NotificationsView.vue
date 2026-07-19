<template>
  <div class="page-card">
    <div class="toolbar">
      <div>
        <h3 style="margin: 0">消息中心</h3>
        <p class="muted" style="margin: 6px 0 0">申请结果、排班变更通知会在这里汇总</p>
      </div>
      <div>
        <el-button @click="load" :loading="loading">刷新</el-button>
        <el-button type="primary" @click="markAll">全部已读</el-button>
      </div>
    </div>

    <el-empty v-if="!loading && !items.length" description="暂无消息" />
    <div v-else class="list">
      <div v-for="item in items" :key="item.id" class="item" :class="{ unread: !item.isRead }" @click="markOne(item)">
        <div class="item-title">
          <span>{{ item.title }}</span>
          <el-tag v-if="!item.isRead" size="small" type="danger">未读</el-tag>
        </div>
        <div class="item-content">{{ item.content }}</div>
        <div class="muted">{{ formatTime(item.createdAt) }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import dayjs from 'dayjs';
import { ElMessage } from 'element-plus';
import {
  markAllNotificationsReadApi,
  markNotificationReadApi,
  notificationsApi,
  type NotificationItem,
} from '../api';
import { onRealtime } from '../realtime';

const loading = ref(false);
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
.list {
  display: grid;
  gap: 12px;
}

.item {
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 14px 16px;
  background: #fff;
  cursor: pointer;
}

.item.unread {
  border-color: #fca5a5;
  background: #fff7f7;
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
}
</style>
