import {
  BadRequestException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Not, Repository } from 'typeorm';
import { ShiftTemplate } from './shift-template.entity';
import { CreateShiftDto } from './dto/create-shift.dto';
import { UpdateShiftDto } from './dto/update-shift.dto';
import { Schedule } from '../schedules/schedule.entity';
import { RealtimeGateway } from '../realtime/realtime.gateway';

@Injectable()
export class ShiftsService {
  constructor(
    @InjectRepository(ShiftTemplate)
    private readonly shiftsRepo: Repository<ShiftTemplate>,
    @InjectRepository(Schedule)
    private readonly schedulesRepo: Repository<Schedule>,
    private readonly realtimeGateway: RealtimeGateway,
  ) {}

  list(storeId: string) {
    return this.shiftsRepo.find({
      where: { storeId },
      order: { sortOrder: 'ASC', createdAt: 'ASC' },
    });
  }

  private normalizeCode(name: string, code?: string) {
    const raw = (code || name || '').trim().toUpperCase();
    const normalized = raw
      .replace(/\s+/g, '_')
      .replace(/[^A-Z0-9_\u4e00-\u9fa5]/g, '')
      .slice(0, 32);
    return normalized || `SHIFT_${Date.now()}`;
  }

  private async ensureUniqueCode(storeId: string, code: string, excludeId?: string) {
    const existing = await this.shiftsRepo.findOne({
      where: excludeId
        ? { storeId, code, id: Not(excludeId) }
        : { storeId, code },
    });
    if (existing) {
      throw new BadRequestException('班次编码已存在');
    }
  }

  private async nextSortOrder(storeId: string) {
    const last = await this.shiftsRepo.find({
      where: { storeId },
      order: { sortOrder: 'DESC' },
      take: 1,
    });
    return (last[0]?.sortOrder || 0) + 1;
  }

  async create(storeId: string, dto: CreateShiftDto) {
    const code = this.normalizeCode(dto.name, dto.code);
    await this.ensureUniqueCode(storeId, code);

    const shift = await this.shiftsRepo.save(
      this.shiftsRepo.create({
        storeId,
        name: dto.name.trim(),
        code,
        startTime: dto.isOff ? '00:00' : dto.startTime,
        endTime: dto.isOff ? '00:00' : dto.endTime,
        color: dto.color || (dto.isOff ? '#909399' : '#409EFF'),
        sortOrder: dto.sortOrder ?? (await this.nextSortOrder(storeId)),
        isOff: Boolean(dto.isOff),
      }),
    );

    this.realtimeGateway.emitToStore(storeId, 'shifts.updated', {
      action: 'created',
      shiftId: shift.id,
    });
    return shift;
  }

  async update(storeId: string, id: string, dto: UpdateShiftDto) {
    const shift = await this.shiftsRepo.findOne({ where: { id, storeId } });
    if (!shift) {
      throw new NotFoundException('班次不存在');
    }

    if (dto.name !== undefined) shift.name = dto.name.trim();
    if (dto.code !== undefined || dto.name !== undefined) {
      const code = this.normalizeCode(dto.name || shift.name, dto.code || shift.code);
      await this.ensureUniqueCode(storeId, code, shift.id);
      shift.code = code;
    }
    if (dto.startTime !== undefined) shift.startTime = dto.startTime;
    if (dto.endTime !== undefined) shift.endTime = dto.endTime;
    if (dto.color !== undefined) shift.color = dto.color;
    if (dto.sortOrder !== undefined) shift.sortOrder = dto.sortOrder;
    if (dto.isOff !== undefined) {
      shift.isOff = Boolean(dto.isOff);
      if (shift.isOff) {
        shift.startTime = '00:00';
        shift.endTime = '00:00';
      }
    }

    const saved = await this.shiftsRepo.save(shift);
    this.realtimeGateway.emitToStore(storeId, 'shifts.updated', {
      action: 'updated',
      shiftId: saved.id,
    });
    return saved;
  }

  async remove(storeId: string, id: string) {
    const shift = await this.shiftsRepo.findOne({ where: { id, storeId } });
    if (!shift) {
      throw new NotFoundException('班次不存在');
    }

    const usedCount = await this.schedulesRepo.count({
      where: { storeId, shiftTemplateId: id },
    });
    if (usedCount > 0) {
      throw new BadRequestException('该班次已用于排班，无法删除。可改为修改时间，或先调整相关排班');
    }

    const remaining = await this.shiftsRepo.count({ where: { storeId } });
    if (remaining <= 1) {
      throw new BadRequestException('至少保留一个班次');
    }

    await this.shiftsRepo.remove(shift);
    this.realtimeGateway.emitToStore(storeId, 'shifts.updated', {
      action: 'deleted',
      shiftId: id,
    });
    return { ok: true };
  }
}
