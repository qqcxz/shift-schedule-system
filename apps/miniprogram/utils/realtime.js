const { getBaseUrl } = require('./request');

let socketTask = null;
const handlers = {};

function connectSocket(token) {
  if (!token) return;
  if (socketTask) {
    try {
      socketTask.close({});
    } catch (e) {
      // ignore
    }
    socketTask = null;
  }

  // 小程序原生 WebSocket 不能直接走 Socket.IO 协议握手；
  // 这里采用轮询兜底 + 可选自定义网关。若后续接入 socket.io 小程序适配库可替换。
  // 目前页面通过下拉刷新和 onShow 刷新保证数据最新。
  // 保留接口，避免调用方报错。
  const base = getBaseUrl().replace(/^http/, 'ws');
  try {
    socketTask = wx.connectSocket({
      url: `${base}/socket.io/?EIO=4&transport=websocket`,
      header: {
        Authorization: `Bearer ${token}`,
      },
    });

    socketTask.onOpen(() => {
      // Socket.IO 需要额外 engine.io 协议，这里仅尝试建立连接，失败不影响业务。
    });
    socketTask.onMessage((msg) => {
      // 粗略解析，能识别到业务事件则分发
      try {
        const data = typeof msg.data === 'string' ? msg.data : '';
        if (!data || data[0] !== '4') return;
        // 4 开头是 message packet，后续可能是 2["event", payload]
        const payloadText = data.replace(/^4/, '');
        if (!payloadText.startsWith('2')) return;
        const arr = JSON.parse(payloadText.slice(1));
        const event = arr[0];
        const payload = arr[1];
        (handlers[event] || []).forEach((fn) => fn(payload));
      } catch (e) {
        // ignore parse errors
      }
    });
    socketTask.onClose(() => {
      socketTask = null;
    });
    socketTask.onError(() => {
      // 连接失败时静默，页面仍可用 REST
      socketTask = null;
    });
  } catch (e) {
    socketTask = null;
  }
}

function disconnectSocket() {
  if (socketTask) {
    try {
      socketTask.close({});
    } catch (e) {
      // ignore
    }
  }
  socketTask = null;
}

function onRealtime(event, handler) {
  if (!handlers[event]) handlers[event] = [];
  handlers[event].push(handler);
  return () => {
    handlers[event] = (handlers[event] || []).filter((fn) => fn !== handler);
  };
}

module.exports = {
  connectSocket,
  disconnectSocket,
  onRealtime,
};
