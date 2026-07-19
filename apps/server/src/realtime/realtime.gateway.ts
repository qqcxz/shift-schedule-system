import {
  ConnectedSocket,
  MessageBody,
  OnGatewayConnection,
  SubscribeMessage,
  WebSocketGateway,
  WebSocketServer,
} from '@nestjs/websockets';
import { JwtService } from '@nestjs/jwt';
import { Server, Socket } from 'socket.io';

type AuthPayload = {
  sub: string;
  role: string;
  storeId: string;
  username: string;
};

@WebSocketGateway({
  cors: {
    origin: (process.env.CORS_ORIGIN || 'http://localhost:5173').split(',').map((item) => item.trim()),
    credentials: true,
  },
})
export class RealtimeGateway implements OnGatewayConnection {
  @WebSocketServer()
  server!: Server;

  constructor(private readonly jwtService: JwtService) {}

  async handleConnection(client: Socket) {
    try {
      const token =
        (client.handshake.auth?.token as string | undefined) ||
        (client.handshake.headers.authorization || '').replace(/^Bearer\s+/i, '');
      if (!token) {
        client.disconnect(true);
        return;
      }

      const payload = await this.jwtService.verifyAsync<AuthPayload>(token, {
        secret: process.env.JWT_SECRET || 'dev-secret-change-me',
      });

      client.data.user = {
        userId: payload.sub,
        role: payload.role,
        storeId: payload.storeId,
        username: payload.username,
      };

      client.join(`store:${payload.storeId}`);
      client.join(`user:${payload.sub}`);
      client.emit('connected', { ok: true, userId: payload.sub });
    } catch {
      client.disconnect(true);
    }
  }

  @SubscribeMessage('ping')
  handlePing(@ConnectedSocket() client: Socket, @MessageBody() data: unknown) {
    client.emit('pong', data || { time: Date.now() });
  }

  emitToStore(storeId: string, event: string, payload: unknown) {
    this.server?.to(`store:${storeId}`).emit(event, payload);
  }

  emitToUser(userId: string, event: string, payload: unknown) {
    this.server?.to(`user:${userId}`).emit(event, payload);
  }
}
