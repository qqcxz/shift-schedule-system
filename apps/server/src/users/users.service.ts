import { Injectable, NotFoundException } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { User } from './user.entity';

@Injectable()
export class UsersService {
  constructor(@InjectRepository(User) private readonly usersRepo: Repository<User>) {}

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

  findByStore(storeId: string) {
    return this.usersRepo.find({
      where: { storeId, isActive: true },
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
    };
  }
}
