import { io, Socket } from 'socket.io-client';

let socket: Socket | null = null;

export function connectSocket(token: string) {
  if (socket) {
    socket.disconnect();
  }

  socket = io('/', {
    path: '/socket.io',
    auth: { token },
    transports: ['websocket'],
  });

  return socket;
}

export function getSocket() {
  return socket;
}

export function disconnectSocket() {
  socket?.disconnect();
  socket = null;
}

export function onRealtime(event: string, handler: (payload: any) => void) {
  socket?.on(event, handler);
  return () => socket?.off(event, handler);
}
