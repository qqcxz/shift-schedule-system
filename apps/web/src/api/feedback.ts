import { ElMessage } from 'element-plus';
import { isHandledApiError } from './errors';

// Session expiration is reported once by the central handler, not once per
// failed request/page. All other errors keep their normal page-level feedback.
export function showApiError(error: unknown) {
  if (isHandledApiError(error)) return;
  ElMessage.error(error instanceof Error ? error.message : '请求失败');
}
