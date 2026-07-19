import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Notification } from './notification.entity';
import { User, UserRole } from '../users/user.entity';
import { RealtimeGateway } from '../realtime/realtime.gateway';

@Injectable()
export class NotificationsService {
  constructor(
    @InjectRepository(Notification)
    private readonly notificationsRepo: Repository<Notification>,
    @InjectRepository(User)
    private readonly usersRepo: Repository<User>,
    private readonly realtimeGateway: RealtimeGateway,
  ) {}

  async create(
    storeId: string,
    userId: string,
    title: string,
    content: string,
    relatedType?: string,
    relatedId?: string,
  ) {
    const row = await this.notificationsRepo.save(
      this.notificationsRepo.create({
        storeId,
        userId,
        title,
        content,
        relatedType,
        relatedId,
      }),
    );

    this.realtimeGateway.emitToUser(userId, 'notification.created', row);
    return row;
  }

  async notifyManagers(
    storeId: string,
    title: string,
    content: string,
    relatedType?: string,
    relatedId?: string,
  ) {
    const managers = await this.usersRepo.find({
      where: { storeId, role: UserRole.MANAGER, isActive: true },
    });

    const rows = [];
    for (const manager of managers) {
      rows.push(
        await this.create(storeId, manager.id, title, content, relatedType, relatedId),
      );
    }
    return rows;
  }

  list(userId: string) {
    return this.notificationsRepo.find({
      where: { userId },
      order: { createdAt: 'DESC' },
      take: 50,
    });
  }

  async markRead(userId: string, id: string) {
    const row = await this.notificationsRepo.findOne({ where: { id, userId } });
    if (!row) {
      return null;
    }
    row.isRead = true;
    return this.notificationsRepo.save(row);
  }

  async markAllRead(userId: string) {
    await this.notificationsRepo.update({ userId, isRead: false }, { isRead: true });
    return { ok: true };
  }
}

