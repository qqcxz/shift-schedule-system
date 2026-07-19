import { Module, forwardRef } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ScheduleRequest } from './schedule-request.entity';
import { RequestsService } from './requests.service';
import { RequestsController } from './requests.controller';
import { SchedulesModule } from '../schedules/schedules.module';
import { RealtimeModule } from '../realtime/realtime.module';
import { NotificationsModule } from '../notifications/notifications.module';
import { ShiftTemplate } from '../shifts/shift-template.entity';

@Module({
  imports: [
    TypeOrmModule.forFeature([ScheduleRequest, ShiftTemplate]),
    forwardRef(() => SchedulesModule),
    forwardRef(() => RealtimeModule),
    NotificationsModule,
  ],
  providers: [RequestsService],
  controllers: [RequestsController],
  exports: [RequestsService],
})
export class RequestsModule {}
