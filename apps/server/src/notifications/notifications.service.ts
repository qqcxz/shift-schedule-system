import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { In, LessThan, Repository } from 'typeorm';
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

    // Keep inbox from growing forever for each user.
    await this.trimUserInbox(userId);
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
      take: 100,
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

  async remove(userId: string, id: string) {
    const result = await this.notificationsRepo.delete({ id, userId });
    return { ok: true, deleted: result.affected || 0 };
  }

  async clear(userId: string, mode: 'read' | 'all' = 'read') {
    if (mode === 'all') {
      const result = await this.notificationsRepo.delete({ userId });
      return { ok: true, deleted: result.affected || 0, mode };
    }

    const result = await this.notificationsRepo.delete({ userId, isRead: true });
    return { ok: true, deleted: result.affected || 0, mode: 'read' };
  }

  private async trimUserInbox(userId: string, keep = 100) {
    const overflow = await this.notificationsRepo.find({
      where: { userId },
      order: { createdAt: 'DESC' },
      skip: keep,
      take: 500,
      select: ['id'],
    });
    if (!overflow.length) {
      return;
    }
    await this.notificationsRepo.delete({
      id: In(overflow.map((item) => item.id)),
    });

    // Also drop very old read notifications automatically.
    const cutoff = new Date();
    cutoff.setDate(cutoff.getDate() - 30);
    await this.notificationsRepo.delete({
      userId,
      isRead: true,
      createdAt: LessThan(cutoff),
    });
  }
}
