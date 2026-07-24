import {
  BadRequestException,
  Injectable,
  NotFoundException,
} from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import * as bcrypt from 'bcryptjs';
import { Not, Repository } from 'typeorm';
import { User, UserRole } from './user.entity';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { RealtimeGateway } from '../realtime/realtime.gateway';

@Injectable()
export class UsersService {
  constructor(
    @InjectRepository(User) private readonly usersRepo: Repository<User>,
    private readonly realtimeGateway: RealtimeGateway,
  ) {}

  findByUsername(username: string) {
    return this.usersRepo.findOne({ where: { username } });
  }

  async findById(id: string) {
    const user = await this.usersRepo.findOne({ where: { id } });
    if (!user) {
      throw new NotFoundException('用户不存在');
    }
    return user;
  }

  findByStore(storeId: string, includeInactive = false) {
    return this.usersRepo.find({
      where: includeInactive ? { storeId } : { storeId, isActive: true },
      order: { role: 'ASC', displayName: 'ASC' },
    });
  }

  sanitize(user: User) {
    return {
      id: user.id,
      username: user.username,
      displayName: user.displayName,
      role: user.role,
      storeId: user.storeId,
      isActive: user.isActive,
    };
  }

  private normalizeUsername(username: string) {
    return username.trim().toLowerCase();
  }

  private async ensureUniqueUsername(username: string, excludeId?: string) {
    const existing = await this.usersRepo.findOne({
      where: excludeId
        ? { username, id: Not(excludeId) }
        : { username },
    });
    if (existing) {
      throw new BadRequestException('用户名已存在');
    }
  }

  async create(storeId: string, dto: CreateUserDto) {
    const username = this.normalizeUsername(dto.username);
    if (!/^[a-z0-9_\u4e00-\u9fa5.-]{2,32}$/i.test(username)) {
      throw new BadRequestException('用户名仅支持字母、数字、下划线、中文、. -');
    }
    await this.ensureUniqueUsername(username);

    const passwordHash = await bcrypt.hash(dto.password, 10);
    const user = await this.usersRepo.save(
      this.usersRepo.create({
        username,
        displayName: dto.displayName.trim(),
        passwordHash,
        role: dto.role || UserRole.STAFF,
        storeId,
        isActive: true,
      }),
    );

    this.realtimeGateway.emitToStore(storeId, 'users.updated', {
      action: 'created',
      userId: user.id,
    });
    return this.sanitize(user);
  }

  async update(storeId: string, id: string, operatorId: string, dto: UpdateUserDto) {
    const user = await this.usersRepo.findOne({ where: { id, storeId } });
    if (!user) {
      throw new NotFoundException('用户不存在');
    }

    if (dto.username !== undefined) {
      const username = this.normalizeUsername(dto.username);
      if (!/^[a-z0-9_\u4e00-\u9fa5.-]{2,32}$/i.test(username)) {
        throw new BadRequestException('用户名仅支持字母、数字、下划线、中文、. -');
      }
      await this.ensureUniqueUsername(username, user.id);
      user.username = username;
    }

    if (dto.displayName !== undefined) {
      user.displayName = dto.displayName.trim();
    }

    if (dto.password) {
      user.passwordHash = await bcrypt.hash(dto.password, 10);
    }

    if (dto.role !== undefined && dto.role !== user.role) {
      if (user.id === operatorId && dto.role !== UserRole.MANAGER) {
        throw new BadRequestException('不能取消自己的店长权限');
      }
      if (user.role === UserRole.MANAGER && dto.role !== UserRole.MANAGER) {
        const managerCount = await this.usersRepo.count({
          where: { storeId, role: UserRole.MANAGER, isActive: true },
        });
        if (managerCount <= 1) {
          throw new BadRequestException('门店至少保留一名店长');
        }
      }
      user.role = dto.role;
    }

    if (dto.isActive !== undefined && dto.isActive !== user.isActive) {
      if (!dto.isActive) {
        if (user.id === operatorId) {
          throw new BadRequestException('不能停用自己的账号');
        }
        if (user.role === UserRole.MANAGER) {
          const managerCount = await this.usersRepo.count({
            where: { storeId, role: UserRole.MANAGER, isActive: true },
          });
          if (managerCount <= 1) {
            throw new BadRequestException('门店至少保留一名店长');
          }
        }
      }
      user.isActive = dto.isActive;
    }

    const saved = await this.usersRepo.save(user);
    this.realtimeGateway.emitToStore(storeId, 'users.updated', {
      action: 'updated',
      userId: saved.id,
    });
    return this.sanitize(saved);
  }

  async remove(storeId: string, id: string, operatorId: string) {
    const user = await this.usersRepo.findOne({ where: { id, storeId } });
    if (!user) {
      throw new NotFoundException('用户不存在');
    }
    if (user.id === operatorId) {
      throw new BadRequestException('不能删除自己的账号');
    }
    if (user.role === UserRole.MANAGER) {
      const managerCount = await this.usersRepo.count({
        where: { storeId, role: UserRole.MANAGER, isActive: true },
      });
      if (managerCount <= 1) {
        throw new BadRequestException('门店至少保留一名店长');
      }
    }

    // Soft delete: keep historical schedules, hide from active roster.
    user.isActive = false;
    await this.usersRepo.save(user);

    this.realtimeGateway.emitToStore(storeId, 'users.updated', {
      action: 'deleted',
      userId: user.id,
    });
    return { ok: true };
  }
}
