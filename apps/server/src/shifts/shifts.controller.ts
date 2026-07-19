import { Controller, Get, Req, UseGuards } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { ShiftTemplate } from './shift-template.entity';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

@Controller('shifts')
@UseGuards(JwtAuthGuard)
export class ShiftsController {
  constructor(
    @InjectRepository(ShiftTemplate)
    private readonly shiftsRepo: Repository<ShiftTemplate>,
  ) {}

  @Get()
  list(@Req() req: { user: { storeId: string } }) {
    return this.shiftsRepo.find({
      where: { storeId: req.user.storeId },
      order: { sortOrder: 'ASC' },
    });
  }
}
