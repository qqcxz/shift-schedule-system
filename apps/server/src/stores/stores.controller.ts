import { Controller, Get, Req, UseGuards } from '@nestjs/common';
import { InjectRepository } from '@nestjs/typeorm';
import { Repository } from 'typeorm';
import { Store } from './store.entity';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

@Controller('stores')
@UseGuards(JwtAuthGuard)
export class StoresController {
  constructor(@InjectRepository(Store) private readonly storesRepo: Repository<Store>) {}

  @Get('current')
  async current(@Req() req: { user: { storeId: string } }) {
    return this.storesRepo.findOne({ where: { id: req.user.storeId } });
  }
}
