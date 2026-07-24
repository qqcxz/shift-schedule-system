import {
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Patch,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { Roles, RolesGuard } from '../common/roles.decorator';
import { UserRole } from '../users/user.entity';
import { CreateShiftDto } from './dto/create-shift.dto';
import { UpdateShiftDto } from './dto/update-shift.dto';
import { ShiftsService } from './shifts.service';

@Controller('shifts')
@UseGuards(JwtAuthGuard, RolesGuard)
export class ShiftsController {
  constructor(private readonly shiftsService: ShiftsService) {}

  @Get()
  list(@Req() req: { user: { storeId: string } }) {
    return this.shiftsService.list(req.user.storeId);
  }

  @Post()
  @Roles(UserRole.MANAGER)
  create(
    @Req() req: { user: { storeId: string } },
    @Body() dto: CreateShiftDto,
  ) {
    return this.shiftsService.create(req.user.storeId, dto);
  }

  @Patch(':id')
  @Roles(UserRole.MANAGER)
  update(
    @Req() req: { user: { storeId: string } },
    @Param('id') id: string,
    @Body() dto: UpdateShiftDto,
  ) {
    return this.shiftsService.update(req.user.storeId, id, dto);
  }

  @Delete(':id')
  @Roles(UserRole.MANAGER)
  remove(
    @Req() req: { user: { storeId: string } },
    @Param('id') id: string,
  ) {
    return this.shiftsService.remove(req.user.storeId, id);
  }
}
