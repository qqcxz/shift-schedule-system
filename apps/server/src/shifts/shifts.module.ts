import { Module } from '@nestjs/common';
import { TypeOrmModule } from '@nestjs/typeorm';
import { ShiftTemplate } from './shift-template.entity';
import { ShiftsController } from './shifts.controller';

@Module({
  imports: [TypeOrmModule.forFeature([ShiftTemplate])],
  controllers: [ShiftsController],
  exports: [TypeOrmModule],
})
export class ShiftsModule {}
