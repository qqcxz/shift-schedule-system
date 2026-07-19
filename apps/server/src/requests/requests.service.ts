import {
  BadRequestException,
  ForbiddenException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import {
  RequestStatus,
  RequestType,
  ScheduleRequest,
} from './schedule-request.entity';
import { CreateRequestDto } from './dto/create-request.dto';
import { ReviewRequestDto } from './dto/review-request.dto';
import { SchedulesService } from '../schedules/schedules.service';
import { RealtimeGateway } from '../realtime/realtime.gateway';
import { NotificationsService } from '../notifications/notifications.service';
import { ShiftTemplate } from '../shifts/shift-template.entity';
import { UserRole } from '../users/user.entity';

@Injectable()
export class RequestsService {
  constructor(
    @InjectRepository(ScheduleRequest)
    private readonly requestsRepo: Repository<ScheduleRequest>,
    @InjectRepository(ShiftTemplate)
    private readonly shiftsRepo: Repository<ShiftTemplate>,
    private readonly schedulesService: SchedulesService,
    private readonly realtimeGateway: RealtimeGateway,
    private readonly notificationsService: NotificationsService,
  ) {}

  async create(
    storeId: string,
    requesterId: string,
    role: UserRole,
    dto: CreateRequestDto,
  ) {
    if (role !== UserRole.STAFF && role !== UserRole.MANAGER) {
      throw new ForbiddenException();
    }

    if (dto.type === RequestType.SWAP && !dto.targetUserId) {
      throw new BadRequestException('调班申请必须指定目标店员');
    }
    if (dto.type === RequestType.CHANGE && !dto.toShiftId) {
      throw new BadRequestException('改班申请必须指定目标班次');
    }

    const entity = this.requestsRepo.create({
      storeId,
      type: dto.type,
      requesterId,
      targetUserId: dto.targetUserId,
      fromDate: dto.fromDate,
      toDate: dto.toDate,
      fromShiftId: dto.fromShiftId,
      toShiftId: dto.toShiftId,
      reason: dto.reason,
      status: RequestStatus.PENDING,
    });

    const saved = await this.requestsRepo.save(entity);
    const full = await this.findOne(storeId, saved.id);

    await this.notificationsService.notifyManagers(
      storeId,
      '新的排班申请',
      `${full.requester.displayName} 提交了${this.typeLabel(full.type)}申请`,
      'request',
      full.id,
    );

    this.realtimeGateway.emitToStore(storeId, 'request.created', {
      id: full.id,
      type: full.type,
      status: full.status,
    });

    return full;
  }

  async list(storeId: string, userId: string, role: UserRole, status?: RequestStatus) {
    const where: Record<string, unknown> = { storeId };
    if (status) {
      where.status = status;
    }
    if (role === UserRole.STAFF) {
      where.requesterId = userId;
    }

    return this.requestsRepo.find({
      where,
      order: { createdAt: 'DESC' },
    });
  }

  async findOne(storeId: string, id: string) {
    const item = await this.requestsRepo.findOne({ where: { id, storeId } });
    if (!item) {
      throw new NotFoundException('申请不存在');
    }
    return item;
  }

  async review(
    storeId: string,
    approverId: string,
    id: string,
    dto: ReviewRequestDto,
  ) {
    const request = await this.findOne(storeId, id);
    if (request.status !== RequestStatus.PENDING) {
      throw new BadRequestException('该申请已处理');
    }

    if (dto.action === 'reject') {
      request.status = RequestStatus.REJECTED;
      request.reviewNote = dto.reviewNote;
      request.approverId = approverId;
      request.reviewedAt = new Date();
      const saved = await this.requestsRepo.save(request);

      await this.notificationsService.create(
        storeId,
        request.requesterId,
        '申请已驳回',
        `你的${this.typeLabel(request.type)}申请已被驳回`,
        'request',
        request.id,
      );

      this.realtimeGateway.emitToStore(storeId, 'request.resolved', {
        id: saved.id,
        status: saved.status,
      });
      return saved;
    }

    // approve and mutate schedule in one business flow
    if (request.type === RequestType.LEAVE) {
      const offShift = await this.shiftsRepo.findOne({
        where: { storeId, isOff: true },
      });
      if (!offShift) {
        throw new BadRequestException('未配置休息班次');
      }
      await this.schedulesService.applyLeave(
        storeId,
        request.requesterId,
        request.fromDate,
        offShift.id,
        approverId,
      );
    } else if (request.type === RequestType.SWAP) {
      if (!request.targetUserId) {
        throw new BadRequestException('调班申请缺少目标店员');
      }
      await this.schedulesService.applySwap(
        storeId,
        request.requesterId,
        request.targetUserId,
        request.fromDate,
        approverId,
      );
    } else if (request.type === RequestType.CHANGE) {
      if (!request.toShiftId) {
        throw new BadRequestException('改班申请缺少目标班次');
      }
      await this.schedulesService.applyChange(
        storeId,
        request.requesterId,
        request.fromDate,
        request.toShiftId,
        approverId,
      );
    }

    request.status = RequestStatus.APPROVED;
    request.reviewNote = dto.reviewNote;
    request.approverId = approverId;
    request.reviewedAt = new Date();
    const saved = await this.requestsRepo.save(request);

    const month = request.fromDate.slice(0, 7);
    this.realtimeGateway.emitToStore(storeId, 'schedule.updated', {
      month,
      storeId,
      reason: 'request-approved',
      requestId: saved.id,
    });
    this.realtimeGateway.emitToStore(storeId, 'request.resolved', {
      id: saved.id,
      status: saved.status,
    });

    await this.notificationsService.create(
      storeId,
      request.requesterId,
      '申请已通过',
      `你的${this.typeLabel(request.type)}申请已通过，排班已自动更新`,
      'request',
      request.id,
    );

    if (request.targetUserId) {
      await this.notificationsService.create(
        storeId,
        request.targetUserId,
        '排班已调整',
        `与你相关的调班申请已通过，请查看最新排班`,
        'request',
        request.id,
      );
    }

    return saved;
  }

  private typeLabel(type: RequestType) {
    if (type === RequestType.LEAVE) return '调休';
    if (type === RequestType.SWAP) return '调班';
    return '改班';
  }
}
