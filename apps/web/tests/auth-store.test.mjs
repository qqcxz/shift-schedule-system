import assert from 'node:assert/strict';
import test from 'node:test';
import { createPinia, setActivePinia } from 'pinia';
import { AxiosError } from 'axios';
import { Manager } from 'socket.io-client';
import { useAuthStore } from '../src/stores/auth.ts';
import http, { setSessionExpiredHandler } from '../src/api/http.ts';
import { connectSocket, getSocket, onRealtime } from '../src/realtime.ts';

function setup(t, cachedUser = { id: 'old-user', role: 'manager', mustChangePassword: false }) {
  const originalStorage = globalThis.localStorage;
  const originalAdapter = http.defaults.adapter;
  // Keep real Socket instances and subscriptions without opening network connections
  // or writing outgoing packets to a transport during the simulated recovery.
  t.mock.method(Manager.prototype, 'open', function () { return this; });
  t.mock.method(Manager.prototype, '_packet', () => {});
  const values = new Map([['shift_token', 'old-token']]);
  if (cachedUser) values.set('shift_user', JSON.stringify(cachedUser));
  globalThis.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: (key) => values.delete(key),
  };
  setActivePinia(createPinia());
  const auth = useAuthStore();
  let expirations = 0;
  setSessionExpiredHandler(() => { expirations++; });
  t.after(() => {
    auth.logout();
    http.defaults.adapter = originalAdapter;
    setSessionExpiredHandler(() => {});
    globalThis.localStorage = originalStorage;
  });
  return { auth, values, expirations: () => expirations };
}

function response(config, data) {
  return { config, status: 200, statusText: 'OK', headers: {}, data };
}

const newUser = { id: 'new-user', role: 'staff', mustChangePassword: true };

test('real auth store: a late successful restore cannot overwrite a newer login', async (t) => {
  const { auth, values } = setup(t);
  let finishRestore;
  let signalStarted;
  const started = new Promise((resolve) => { signalStarted = resolve; });
  http.defaults.adapter = (config) => {
    if (config.url === '/auth/login') {
      return Promise.resolve(response(config, { accessToken: 'new-token', user: newUser }));
    }
    return new Promise((resolve) => {
      finishRestore = () => resolve(response(config, { id: 'old-user', role: 'manager' }));
      signalStarted();
    });
  };
  const restoring = auth.restore();
  await started;
  await auth.login('test', 'test-password');
  finishRestore();
  await restoring;
  assert.equal(auth.token, 'new-token');
  assert.equal(auth.user.id, 'new-user');
  assert.equal(values.get('shift_token'), 'new-token');
  assert.equal(JSON.parse(values.get('shift_user')).id, 'new-user');
});

test('real auth store: a late failed restore cannot sign out a newer login', async (t) => {
  const { auth, values, expirations } = setup(t);
  let failRestore;
  let signalStarted;
  const started = new Promise((resolve) => { signalStarted = resolve; });
  http.defaults.adapter = (config) => {
    if (config.url === '/auth/login') {
      return Promise.resolve(response(config, { accessToken: 'new-token', user: newUser }));
    }
    return new Promise((resolve, reject) => {
      failRestore = () => reject(new AxiosError('Unauthorized', undefined, config, undefined, {
        ...response(config, { message: 'Unauthorized' }), status: 401,
      }));
      signalStarted();
    });
  };
  const restoring = auth.restore();
  await started;
  await auth.login('test', 'test-password');
  failRestore();
  await restoring;
  assert.equal(auth.token, 'new-token');
  assert.equal(auth.user.id, 'new-user');
  assert.equal(values.get('shift_token'), 'new-token');
  assert.equal(expirations(), 0);
});

for (const failure of ['network', 'server']) {
  test(`real auth store: restore ${failure} errors retain the session and restore realtime`, async (t) => {
    const { auth, values, expirations } = setup(t);
    http.defaults.adapter = (config) => Promise.reject(failure === 'network'
      ? new Error('Network Error')
      : new AxiosError('Server Error', undefined, config, undefined, {
        ...response(config, { message: 'Server Error' }), status: 500,
      }));
    await auth.restore();
    assert.equal(auth.token, 'old-token');
    assert.equal(auth.user.id, 'old-user');
    assert.equal(values.get('shift_token'), 'old-token');
    assert.equal(expirations(), 0);
    const socket = getSocket();
    assert.ok(socket, 'retaining a cached session must also initialize realtime');
    assert.deepEqual(socket.auth, { token: 'old-token' });
    assert.equal(socket.active, true);
    assert.equal(socket.io.reconnection(), true);

    const updates = [];
    const unsubscribe = onRealtime('schedule.updated', (payload) => updates.push(payload));
    t.after(unsubscribe);
    // Retrying restore must not replace the socket and lose mounted-page listeners.
    await auth.restore();
    assert.equal(getSocket(), socket);
    http.defaults.adapter = (config) => Promise.resolve(response(config, { items: [] }));
    await http.get('/schedules');
    assert.equal(getSocket(), socket);
    // Simulate transport recovery and an incoming event through the real Socket.
    socket.io.emitReserved('packet', { type: 0, nsp: socket.nsp, data: { sid: 'recovered-socket' } });
    socket.io.emitReserved('packet', { type: 2, nsp: socket.nsp, data: ['schedule.updated', { month: '2026-10' }] });
    assert.deepEqual(updates, [{ month: '2026-10' }]);
  });
}

test('real auth store: restore 401 clears both stored credentials and reactive state', async (t) => {
  const { auth, values, expirations } = setup(t);
  http.defaults.adapter = (config) => Promise.reject(new AxiosError(
    'Unauthorized', undefined, config, undefined,
    { ...response(config, { message: 'Unauthorized' }), status: 401 },
  ));
  await auth.restore();
  assert.equal(auth.token, '');
  assert.equal(auth.user, null);
  assert.equal(auth.isLoggedIn, false);
  assert.equal(values.has('shift_token'), false);
  assert.equal(values.has('shift_user'), false);
  assert.equal(expirations(), 1);
  assert.equal(getSocket(), null);
});

test('real auth store: a failed restore does not connect without a complete eligible session', async (t) => {
  for (const cachedUser of [null, { id: 'old-user', role: 'staff', mustChangePassword: true }]) {
    await t.test(cachedUser ? 'password change required' : 'cached user missing', async (t) => {
      const { auth } = setup(t, cachedUser);
      http.defaults.adapter = () => Promise.reject(new Error('Network Error'));
      await auth.restore();
      assert.equal(getSocket(), null);
      assert.equal(auth.token, 'old-token');
    });
  }
});

test('real auth store: a late network failure does not replace a newer login socket', async (t) => {
  const { auth, expirations } = setup(t);
  let failRestore;
  let signalStarted;
  const started = new Promise((resolve) => { signalStarted = resolve; });
  http.defaults.adapter = (config) => {
    if (config.url === '/auth/login') {
      return Promise.resolve(response(config, {
        accessToken: 'new-token', user: { ...newUser, mustChangePassword: false },
      }));
    }
    return new Promise((resolve, reject) => {
      failRestore = () => reject(new Error('Network Error'));
      signalStarted();
    });
  };
  const restoring = auth.restore();
  await started;
  await auth.login('test', 'test-password');
  const newSocket = getSocket();
  assert.ok(newSocket);
  failRestore();
  await restoring;
  assert.equal(auth.token, 'new-token');
  assert.equal(getSocket(), newSocket);
  assert.deepEqual(newSocket.auth, { token: 'new-token' });
  assert.equal(newSocket.active, true);
  assert.equal(expirations(), 0);
});

test('real auth store: a late network failure does not connect after logout', async (t) => {
  const { auth } = setup(t);
  let failRestore;
  let signalStarted;
  const started = new Promise((resolve) => { signalStarted = resolve; });
  http.defaults.adapter = () => new Promise((resolve, reject) => {
    failRestore = () => reject(new Error('Network Error'));
    signalStarted();
  });
  const restoring = auth.restore();
  await started;
  auth.logout();
  failRestore();
  await restoring;
  assert.equal(auth.isLoggedIn, false);
  assert.equal(getSocket(), null);
});

test('real auth store: restore 401 also disconnects an existing socket', async (t) => {
  const { auth } = setup(t);
  const oldSocket = connectSocket(auth.token);
  http.defaults.adapter = (config) => Promise.reject(new AxiosError(
    'Unauthorized', undefined, config, undefined,
    { ...response(config, { message: 'Unauthorized' }), status: 401 },
  ));
  await auth.restore();
  assert.equal(auth.isLoggedIn, false);
  assert.equal(getSocket(), null);
  assert.equal(oldSocket.active, false);
});
