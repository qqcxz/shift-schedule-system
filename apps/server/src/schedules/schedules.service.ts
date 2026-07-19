import {
  BadRequestException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Between, In, Repository } from 'typeorm';
import { Schedule } from './schedule.entity';
import { SaveMonthScheduleDto } from './dto/save-month-schedule.dto';
import { RealtimeGateway } from '../realtime/realtime.gateway';
import { ShiftTemplate } from '../shifts/shift-template.entity';
import { UserRole } from '../users/user.entity';

@Injectable()
export class SchedulesService {
  constructor(
    @InjectRepository(Schedule) private readonly schedulesRepo: Repository<Schedule>,
    @InjectRepository(ShiftTemplate) private readonly shiftsRepo: Repository<ShiftTemplate>,
    private readonly realtimeGateway: RealtimeGateway,
  ) {}

  private monthRange(month: string) {
    if (!/^\d{4}-\d{2}$/.test(month)) {
      throw new BadRequestException('month 格式应为 YYYY-MM');
    }
    const [yearText, monthText] = month.split('-');
    const year = Number(yearText);
    const monthIndex = Number(monthText) - 1;
    const start = `${month}-01`;
    const endDay = new Date(year, monthIndex + 1, 0).getDate();
    const end = `${month}-${String(endDay).padStart(2, '0')}`;
    return { start, end };
  }

  async getMonth(storeId: string, month: string, userId?: string, role?: UserRole) {
    const { start, end } = this.monthRange(month);
    void userId;
    void role;

    const items = await this.schedulesRepo.find({
      where: {
        storeId,
        workDate: Between(start, end),
      },
      relations: ['user', 'shiftTemplate'],
      order: { workDate: 'ASC' },
    });

    return {
      month,
      items: items.map((item) => ({
        id: item.id,
        userId: item.userId,
        userName: item.user?.displayName,
        workDate: item.workDate,
        shiftTemplateId: item.shiftTemplateId,
        shiftName: item.shiftTemplate?.name,
        shiftColor: item.shiftTemplate?.color,
        status: item.status,
        version: item.version,
        note: item.note,
      })),
    };
  }

  async saveMonth(storeId: string, operatorId: string, dto: SaveMonthScheduleDto) {
    const shiftIds = [...new Set(dto.items.map((item) => item.shiftTemplateId))];
    const shifts = await this.shiftsRepo.find({ where: { id: In(shiftIds) } });
    if (shifts.length !== shiftIds.length) {
      throw new BadRequestException('存在无效班次');
    }

    for (const item of dto.items) {
      const existing = await this.schedulesRepo.findOne({
        where: {
          storeId,
          userId: item.userId,
          workDate: item.workDate,
        },
      });

      if (!existing) {
        await this.schedulesRepo
          .createQueryBuilder()
          .insert()
          .into(Schedule)
          .values({
            storeId,
            userId: item.userId,
            workDate: item.workDate,
            shiftTemplateId: item.shiftTemplateId,
            status: item.status || 'normal',
            note: item.note,
            createdById: operatorId,
            updatedById: operatorId,
            version: 1,
          })
          .execute();
        continue;
      }

      if (item.version && item.version !== existing.version) {
        throw new BadRequestException(
          `排班已被其他人更新：${item.workDate} / ${item.userId}`,
        );
      }

      await this.schedulesRepo
        .createQueryBuilder()
        .update(Schedule)
        .set({
          shiftTemplateId: item.shiftTemplateId,
          status: item.status || existing.status,
          note: item.note,
          updatedById: operatorId,
          version: existing.version + 1,
        })
        .where('id = :id', { id: existing.id })
        .execute();
    }

    this.realtimeGateway.emitToStore(storeId, 'schedule.updated', {
      month: dto.month,
      storeId,
      count: dto.items.length,
    });

    return this.getMonth(storeId, dto.month);
  }

  async getUserDate(storeId: string, userId: string, workDate: string) {
    return this.schedulesRepo.findOne({
      where: { storeId, userId, workDate },
    });
  }

  private async updateSchedule(
    id: string,
    patch: {
      shiftTemplateId: string;
      status: string;
      updatedById: string;
      version: number;
    },
  ) {
    await this.schedulesRepo
      .createQueryBuilder()
      .update(Schedule)
      .set(patch)
      .where('id = :id', { id })
      .execute();

    return this.schedulesRepo.findOne({
      where: { id },
      relations: ['shiftTemplate', 'user'],
    });
  }

  async applyLeave(
    storeId: string,
    userId: string,
    workDate: string,
    offShiftId: string,
    operatorId: string,
  ) {
    const row = await this.getUserDate(storeId, userId, workDate);
    if (!row) {
      await this.schedulesRepo
        .createQueryBuilder()
        .insert()
        .into(Schedule)
        .values({
          storeId,
          userId,
          workDate,
          shiftTemplateId: offShiftId,
          status: 'leave',
          createdById: operatorId,
          updatedById: operatorId,
          version: 1,
        })
        .execute();
      return this.getUserDate(storeId, userId, workDate);
    }

    return this.updateSchedule(row.id, {
      shiftTemplateId: offShiftId,
      status: 'leave',
      updatedById: operatorId,
      version: row.version + 1,
    });
  }

  async applySwap(
    storeId: string,
    requesterId: string,
    targetUserId: string,
    workDate: string,
    operatorId: string,
  ) {
    const requester = await this.getUserDate(storeId, requesterId, workDate);
    const target = await this.getUserDate(storeId, targetUserId, workDate);
    if (!requester || !target) {
      throw new NotFoundException('调班双方当天排班不完整');
    }

    const requesterShift = requester.shiftTemplateId;
    const targetShift = target.shiftTemplateId;

    await this.updateSchedule(requester.id, {
      shiftTemplateId: targetShift,
      status: 'swapped',
      updatedById: operatorId,
      version: requester.version + 1,
    });
    await this.updateSchedule(target.id, {
      shiftTemplateId: requesterShift,
      status: 'swapped',
      updatedById: operatorId,
      version: target.version + 1,
    });

    return {
      requester: await this.schedulesRepo.findOne({ where: { id: requester.id } }),
      target: await this.schedulesRepo.findOne({ where: { id: target.id } }),
    };
  }

  async applyChange(
    storeId: string,
    userId: string,
    workDate: string,
    toShiftId: string,
    operatorId: string,
  ) {
    const row = await this.getUserDate(storeId, userId, workDate);
    if (!row) {
      await this.schedulesRepo
        .createQueryBuilder()
        .insert()
        .into(Schedule)
        .values({
          storeId,
          userId,
          workDate,
          shiftTemplateId: toShiftId,
          status: 'normal',
          createdById: operatorId,
          updatedById: operatorId,
          version: 1,
        })
        .execute();
      return this.getUserDate(storeId, userId, workDate);
    }

    return this.updateSchedule(row.id, {
      shiftTemplateId: toShiftId,
      status: 'normal',
      updatedById: operatorId,
      version: row.version + 1,
    });
  }
}
