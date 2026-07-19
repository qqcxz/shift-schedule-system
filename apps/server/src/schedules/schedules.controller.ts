import { Body, Controller, Get, Put, Query, Req, UseGuards } from '@nestjs/common';
import { SchedulesService } from './schedules.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { Roles, RolesGuard } from '../common/roles.decorator';
import { UserRole } from '../users/user.entity';
import { SaveMonthScheduleDto } from './dto/save-month-schedule.dto';

@Controller('schedules')
@UseGuards(JwtAuthGuard, RolesGuard)
export class SchedulesController {
  constructor(private readonly schedulesService: SchedulesService) {}

  @Get()
  getMonth(
    @Req() req: { user: { storeId: string; userId: string; role: UserRole } },
    @Query('month') month: string,
  ) {
    return this.schedulesService.getMonth(req.user.storeId, month, req.user.userId, req.user.role);
  }

  @Put('month')
  @Roles(UserRole.MANAGER)
  saveMonth(
    @Req() req: { user: { storeId: string; userId: string } },
    @Body() dto: SaveMonthScheduleDto,
  ) {
    return this.schedulesService.saveMonth(req.user.storeId, req.user.userId, dto);
  }
}
