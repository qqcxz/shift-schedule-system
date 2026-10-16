import assert from 'node:assert/strict';
import test from 'node:test';
import { AxiosError, AxiosHeaders } from 'axios';
import { createMemoryHistory, createRouter } from 'vue-router';
import { createHttpClient } from '../src/api/http-client.ts';
import { ApiError, isHandledApiError } from '../src/api/errors.ts';
import { getLoginRedirect } from '../src/router/login-redirect.ts';
import { createSessionExpiredHandler } from '../src/router/session-expired.ts';
import { getAuthRedirect, initializeAuth } from '../src/router/auth-guard.ts';

function rejectResponse(config, status, data = { message: 'Unauthorized' }) {
  return Promise.reject(new AxiosError('Request failed', undefined, config, undefined, {
    status,
    statusText: 'Error',
    data,
    headers: new AxiosHeaders(),
    config,
  }));
}

function fixture() {
  const auth = {
    token: 'expired-token',
    user: { id: 'test-user' },
    logoutCount: 0,
    logout() {
      this.token = '';
      this.user = null;
      this.logoutCount++;
    },
  };
  let expiryCount = 0;
  const client = createHttpClient({
    getAuth: () => auth,
    onSessionExpired: () => { expiryCount++; },
  });
  client.defaults.adapter = (config) => rejectResponse(config, 401);
  return { auth, client, expiryCount: () => expiryCount };
}

const isSessionError = (error) => error instanceof ApiError
  && error.status === 401 && error.handled && error.message === '登录已失效，请重新登录';

test('protected 401 clears the session and calls the expiration handler', async () => {
  const { auth, client, expiryCount } = fixture();
  await assert.rejects(client.get('/schedules'), isSessionError);
  assert.equal(auth.token, '');
  assert.equal(auth.user, null);
  assert.equal(auth.logoutCount, 1);
  assert.equal(expiryCount(), 1);
});

test('concurrent and subsequent unauthenticated 401s notify and logout only once', async () => {
  const { auth, client, expiryCount } = fixture();
  const results = await Promise.allSettled([
    client.get('/users'), client.get('/shifts'), client.get('/schedules'),
  ]);
  for (const result of results) {
    assert.equal(result.status, 'rejected');
    assert.ok(isSessionError(result.reason));
    assert.ok(isHandledApiError(result.reason));
  }
  await assert.rejects(client.get('/notifications'), isSessionError);
  assert.equal(auth.logoutCount, 1);
  assert.equal(expiryCount(), 1);
});

test('login 401 keeps its credential error and never clears or sends the cached token', async () => {
  const { auth, client, expiryCount } = fixture();
  client.defaults.adapter = (config) => {
    assert.equal(config.headers.get('Authorization'), undefined);
    return rejectResponse(config, 401, { message: '用户名或密码错误' });
  };
  await assert.rejects(client.post('/auth/login', {}, { skipAuth: true }), (error) => {
    assert.equal(error.message, '用户名或密码错误');
    assert.equal(error.status, 401);
    assert.equal(error.handled, false);
    return true;
  });
  assert.equal(auth.token, 'expired-token');
  assert.equal(auth.logoutCount, 0);
  assert.equal(expiryCount(), 0);
});

test('403, 500 and network failures do not expire a session and retain status/message', async () => {
  const { auth, client, expiryCount } = fixture();
  for (const status of [403, 500]) {
    client.defaults.adapter = (config) => rejectResponse(config, status, { message: '测试错误' });
    await assert.rejects(client.get('/users'), (error) => {
      assert.equal(error.status, status);
      assert.equal(error.message, '测试错误');
      assert.equal(error.handled, false);
      return true;
    });
  }
  client.defaults.adapter = () => Promise.reject(new Error('Network Error'));
  await assert.rejects(client.get('/auth/me'), (error) => {
    assert.equal(error.status, undefined);
    assert.equal(error.message, 'Network Error');
    assert.equal(error.handled, false);
    return true;
  });
  assert.equal(auth.token, 'expired-token');
  assert.equal(auth.logoutCount, 0);
  assert.equal(expiryCount(), 0);
});

test('a late 401 from a previous token cannot sign out a new login', async () => {
  const { auth, client, expiryCount } = fixture();
  let finish;
  let sent;
  const started = new Promise((resolve) => { sent = resolve; });
  client.defaults.adapter = (config) => new Promise((resolve, reject) => {
    finish = () => rejectResponse(config, 401).catch(reject);
    sent();
  });
  const request = client.get('/schedules');
  await started;
  auth.token = 'new-token';
  finish();
  await assert.rejects(request, isSessionError);
  assert.equal(auth.token, 'new-token');
  assert.equal(auth.logoutCount, 0);
  assert.equal(expiryCount(), 0);
});

test('a newly logged-in session can expire independently of the previous session', async () => {
  const { auth, client, expiryCount } = fixture();
  await assert.rejects(client.get('/users'), isSessionError);
  auth.token = 'new-token';
  await assert.rejects(client.get('/users'), isSessionError);
  assert.equal(auth.logoutCount, 2);
  assert.equal(expiryCount(), 2);
});

test('a protected 401 with no cached token still sends the user to login', async () => {
  const { auth, client, expiryCount } = fixture();
  auth.token = '';
  await assert.rejects(client.get('/auth/me'), isSessionError);
  assert.equal(expiryCount(), 1);
});

test('export/blob 401 triggers expiration before parsing and other blob errors remain readable', async () => {
  const { auth, client, expiryCount } = fixture();
  client.defaults.adapter = (config) => rejectResponse(
    config, 401, new Blob(['{"message":"Unauthorized"}']),
  );
  await assert.rejects(client.get('/schedules/export', { responseType: 'blob' }), isSessionError);
  assert.equal(auth.logoutCount, 1);
  assert.equal(expiryCount(), 1);
  client.defaults.adapter = (config) => rejectResponse(
    config, 400, new Blob(['{"message":["日期格式错误","请选择月份"]}']),
  );
  await assert.rejects(client.get('/schedules/export', { responseType: 'blob' }), (error) => {
    assert.equal(error.status, 400);
    assert.equal(error.message, '日期格式错误, 请选择月份');
    assert.equal(error.handled, false);
    return true;
  });
});

test('normal successful requests keep the Bearer token and response unchanged', async () => {
  const { auth, client, expiryCount } = fixture();
  client.defaults.adapter = async (config) => {
    assert.equal(config.headers.get('Authorization'), 'Bearer expired-token');
    return { config, status: 200, statusText: 'OK', headers: {}, data: { ok: true } };
  };
  const response = await client.get('/schedules');
  assert.deepEqual(response.data, { ok: true });
  assert.equal(auth.token, 'expired-token');
  assert.equal(expiryCount(), 0);
});

test('session expiration replaces the route, preserves the return URL and reports once', async () => {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: {} },
      { path: '/schedule', component: {} },
    ],
  });
  await router.push('/schedule?month=2026-10#calendar');
  const messages = [];
  const fallbacks = [];
  const expire = createSessionExpiredHandler(router, (m) => messages.push(m), (u) => fallbacks.push(u));
  await expire();
  assert.equal(router.currentRoute.value.path, '/login');
  assert.equal(router.currentRoute.value.query.redirect, '/schedule?month=2026-10#calendar');
  assert.deepEqual(messages, ['登录已失效，请重新登录']);
  assert.deepEqual(fallbacks, []);
  await expire();
  assert.equal(messages.length, 1);
});

test('navigation failure falls back to a full-page login redirect', async () => {
  for (const replace of [async () => { throw new Error('failed'); }, async () => ({ type: 4 })]) {
    const urls = [];
    const router = {
      currentRoute: { value: { path: '/schedule', fullPath: '/schedule?month=2026-10' } },
      replace,
      resolve: () => ({ href: '/login?redirect=%2Fschedule%3Fmonth%3D2026-10' }),
    };
    await createSessionExpiredHandler(router, () => {}, (url) => urls.push(url))();
    assert.deepEqual(urls, ['/login?redirect=%2Fschedule%3Fmonth%3D2026-10']);
  }
});

function startupRouter(auth) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: {}, meta: { public: true } },
      { path: '/change-password', name: 'change-password', component: {} },
      { path: '/schedule', name: 'schedule', component: {} },
      { path: '/staff', name: 'staff', component: {}, meta: { managerOnly: true } },
    ],
  });
  router.beforeEach((to) => getAuthRedirect(to, auth) ?? true);
  return router;
}

test('startup with an expired token lands on login with the complete return URL', async () => {
  const { auth, client } = fixture();
  Object.defineProperty(auth, 'isLoggedIn', { get: () => Boolean(auth.token) });
  auth.mustChangePassword = false;
  auth.isManager = true;
  auth.restore = async () => { await client.get('/auth/me').catch(() => {}); };
  const router = startupRouter(auth);
  const messages = [];
  const replaceCalls = [];
  const replace = router.replace.bind(router);
  router.replace = (target) => { replaceCalls.push(target); return replace(target); };
  const onExpired = createSessionExpiredHandler(router, (m) => messages.push(m), () => assert.fail('unexpected fallback'));
  const startupClient = createHttpClient({ getAuth: () => auth, onSessionExpired: onExpired });
  startupClient.defaults.adapter = (config) => rejectResponse(config, 401);
  auth.restore = async () => {
    await Promise.allSettled([startupClient.get('/auth/me'), startupClient.get('/users')]);
  };
  await router.push('/schedule?month=2026-10#calendar');
  await initializeAuth(router, auth);
  assert.equal(router.currentRoute.value.name, 'login');
  assert.equal(router.currentRoute.value.query.redirect, '/schedule?month=2026-10#calendar');
  assert.equal(auth.logoutCount, 1);
  assert.equal(messages.length, 1);
  assert.equal(replaceCalls.length, 1);
});

test('startup rechecks changed password requirements and role after restoring the user', async () => {
  for (const [initial, restored, expected] of [
    ['/schedule', { mustChangePassword: true }, 'change-password'],
    ['/staff', { isManager: false }, 'schedule'],
    ['/change-password', { mustChangePassword: false }, 'schedule'],
  ]) {
    const auth = {
      isLoggedIn: true,
      mustChangePassword: initial === '/change-password',
      isManager: true,
      restore: async () => { Object.assign(auth, restored); },
    };
    const router = startupRouter(auth);
    await router.push(initial);
    await initializeAuth(router, auth);
    assert.equal(router.currentRoute.value.name, expected);
  }
});

test('late navigation failure must not reload a newly logged-in session', async () => {
  const router = {
    currentRoute: { value: { path: '/schedule', fullPath: '/schedule' } },
    replace: async () => ({ type: 4 }),
    resolve: () => ({ href: '/login' }),
  };
  await createSessionExpiredHandler(router, () => {}, () => assert.fail('new session must not reload'), () => true)();
});

test('login return URL accepts local pages and rejects external URLs and auth loops', () => {
  assert.equal(getLoginRedirect('/requests?status=pending#list'), '/requests?status=pending#list');
  for (const value of [undefined, null, [], 'https://example.com', '//example.com', '/\\example.com', '/login', '/login?redirect=/login', '/change-password', '/LOGIN', '/Change-Password', '/bad\npath']) {
    assert.equal(getLoginRedirect(value), '/schedule');
  }
});
