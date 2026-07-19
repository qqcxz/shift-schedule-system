import { Injectable } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import * as bcrypt from 'bcryptjs';
import { mkdirSync } from 'fs';
import { dirname } from 'path';
import { User, UserRole } from './users/user.entity';
import { Store } from './stores/store.entity';
import { ShiftTemplate } from './shifts/shift-template.entity';
import { Schedule } from './schedules/schedule.entity';

@Injectable()
export class SeedService {
  constructor(
    @InjectRepository(User) private readonly usersRepo: Repository<User>,
    @InjectRepository(Store) private readonly storesRepo: Repository<Store>,
    @InjectRepository(ShiftTemplate) private readonly shiftsRepo: Repository<ShiftTemplate>,
    @InjectRepository(Schedule) private readonly schedulesRepo: Repository<Schedule>,
  ) {}

  async seedIfEmpty() {
    const dbPath = process.env.DB_DATABASE || './data/shift.sqlite';
    if ((process.env.DB_TYPE || 'sqlite').toLowerCase() === 'sqlite') {
      mkdirSync(dirname(dbPath), { recursive: true });
    }

    const userCount = await this.usersRepo.count();
    if (userCount > 0) {
      return;
    }

    const store = await this.storesRepo.save(
      this.storesRepo.create({
        name: '示例门店',
        address: '示例路 1 号',
      }),
    );

    const passwordHash = await bcrypt.hash('123456', 10);
    const manager = await this.usersRepo.save(
      this.usersRepo.create({
        username: 'manager',
        displayName: '店长王明',
        passwordHash,
        role: UserRole.MANAGER,
        storeId: store.id,
      }),
    );

    const staffDefs = [
      { username: 'staff1', displayName: '店员小李' },
      { username: 'staff2', displayName: '店员小张' },
      { username: 'staff3', displayName: '店员小陈' },
    ];

    const staffUsers: User[] = [];
    for (const item of staffDefs) {
      staffUsers.push(
        await this.usersRepo.save(
          this.usersRepo.create({
            username: item.username,
            displayName: item.displayName,
            passwordHash,
            role: UserRole.STAFF,
            storeId: store.id,
          }),
        ),
      );
    }

    const shifts = await this.shiftsRepo.save([
      this.shiftsRepo.create({
        storeId: store.id,
        name: '早班',
        code: 'MORNING',
        startTime: '09:00',
        endTime: '13:00',
        color: '#67C23A',
        sortOrder: 1,
      }),
      this.shiftsRepo.create({
        storeId: store.id,
        name: '中班',
        code: 'NOON',
        startTime: '13:00',
        endTime: '17:00',
        color: '#E6A23C',
        sortOrder: 2,
      }),
      this.shiftsRepo.create({
        storeId: store.id,
        name: '晚班',
        code: 'NIGHT',
        startTime: '17:00',
        endTime: '21:00',
        color: '#409EFF',
        sortOrder: 3,
      }),
      this.shiftsRepo.create({
        storeId: store.id,
        name: '休息',
        code: 'OFF',
        startTime: '00:00',
        endTime: '00:00',
        color: '#909399',
        sortOrder: 4,
        isOff: true,
      }),
    ]);

    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth();
    const daysInMonth = new Date(year, month + 1, 0).getDate();
    const scheduleRows: Schedule[] = [];
    const workShifts = shifts.filter((item) => !item.isOff);
    const offShift = shifts.find((item) => item.isOff)!;

    for (let day = 1; day <= daysInMonth; day += 1) {
      const workDate = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
      staffUsers.forEach((user, index) => {
        const isOff = (day + index) % 7 === 0;
        const shift = isOff ? offShift : workShifts[(day + index) % workShifts.length];
        scheduleRows.push(
          this.schedulesRepo.create({
            storeId: store.id,
            userId: user.id,
            workDate,
            shiftTemplateId: shift.id,
            status: isOff ? 'leave' : 'normal',
            createdById: manager.id,
            updatedById: manager.id,
          }),
        );
      });
    }

    await this.schedulesRepo.save(scheduleRows);
    // eslint-disable-next-line no-console
    console.log('Seed data ready: manager/staff1/staff2/staff3 password=123456');
  }
}
