import { Module, OnModuleInit } from '@nestjs/common';
import { ConfigModule, ConfigService } from '@nestjs/config';
import { TypeOrmModule } from '@nestjs/typeorm';
import { AuthModule } from './auth/auth.module';
import { UsersModule } from './users/users.module';
import { StoresModule } from './stores/stores.module';
import { ShiftsModule } from './shifts/shifts.module';
import { SchedulesModule } from './schedules/schedules.module';
import { RequestsModule } from './requests/requests.module';
import { NotificationsModule } from './notifications/notifications.module';
import { RealtimeModule } from './realtime/realtime.module';
import { User } from './users/user.entity';
import { Store } from './stores/store.entity';
import { ShiftTemplate } from './shifts/shift-template.entity';
import { Schedule } from './schedules/schedule.entity';
import { ScheduleRequest } from './requests/schedule-request.entity';
import { Notification } from './notifications/notification.entity';
import { SeedService } from './seed.service';
import { AppController } from './app.controller';
import { mkdirSync } from 'fs';
import { dirname } from 'path';

@Module({
  imports: [
    ConfigModule.forRoot({ isGlobal: true }),
    TypeOrmModule.forRootAsync({
      inject: [ConfigService],
      useFactory: (config: ConfigService) => {
        const dbType = (config.get<string>('DB_TYPE') || 'sqljs').toLowerCase();
        if (dbType === 'mysql') {
          return {
            type: 'mysql' as const,
            host: config.get<string>('DB_HOST', 'localhost'),
            port: Number(config.get<string>('DB_PORT', '3306')),
            username: config.get<string>('DB_USER', 'shift'),
            password: config.get<string>('DB_PASSWORD', 'shift123'),
            database: config.get<string>('DB_NAME', 'shift_schedule'),
            entities: [User, Store, ShiftTemplate, Schedule, ScheduleRequest, Notification],
            synchronize: true,
          };
        }

        const location = config.get<string>('DB_DATABASE', './data/shift.sqlite');
        mkdirSync(dirname(location), { recursive: true });
        return {
          type: 'sqljs' as const,
          location,
          autoSave: true,
          entities: [User, Store, ShiftTemplate, Schedule, ScheduleRequest, Notification],
          synchronize: true,
        };
      },
    }),
    TypeOrmModule.forFeature([User, Store, ShiftTemplate, Schedule, ScheduleRequest, Notification]),
    AuthModule,
    UsersModule,
    StoresModule,
    ShiftsModule,
    SchedulesModule,
    RequestsModule,
    NotificationsModule,
    RealtimeModule,
  ],
  controllers: [AppController],
  providers: [SeedService],
})
export class AppModule implements OnModuleInit {
  constructor(private readonly seedService: SeedService) {}

  async onModuleInit() {
    if ((process.env.SEED_ON_BOOT || 'true').toLowerCase() === 'true') {
      await this.seedService.seedIfEmpty();
    }
  }
}
