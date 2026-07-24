import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ShiftTemplate } from './shift-template.entity';
import { Schedule } from '../schedules/schedule.entity';
import { ShiftsController } from './shifts.controller';
import { ShiftsService } from './shifts.service';
import { RealtimeModule } from '../realtime/realtime.module';

@Module({
  imports: [TypeOrmModule.forFeature([ShiftTemplate, Schedule]), RealtimeModule],
  controllers: [ShiftsController],
  providers: [ShiftsService],
  exports: [TypeOrmModule, ShiftsService],
})
export class ShiftsModule {}
