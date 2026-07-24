import {
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Patch,
  Post,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { UsersService } from './users.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { Roles, RolesGuard } from '../common/roles.decorator';
import { UserRole } from './user.entity';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';

@Controller('users')
@UseGuards(JwtAuthGuard, RolesGuard)
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Get()
  async list(
    @Req() req: { user: { storeId: string; role: UserRole } },
    @Query('includeInactive') includeInactive?: string,
  ) {
    const allowInactive =
      req.user.role === UserRole.MANAGER &&
      (includeInactive === '1' || includeInactive === 'true');
    const users = await this.usersService.findByStore(req.user.storeId, allowInactive);
    return users.map((user) => this.usersService.sanitize(user));
  }

  @Post()
  @Roles(UserRole.MANAGER)
  create(
    @Req() req: { user: { storeId: string } },
    @Body() dto: CreateUserDto,
  ) {
    return this.usersService.create(req.user.storeId, dto);
  }

  @Patch(':id')
  @Roles(UserRole.MANAGER)
  update(
    @Req() req: { user: { storeId: string; userId: string } },
    @Param('id') id: string,
    @Body() dto: UpdateUserDto,
  ) {
    return this.usersService.update(req.user.storeId, id, req.user.userId, dto);
  }

  @Delete(':id')
  @Roles(UserRole.MANAGER)
  remove(
    @Req() req: { user: { storeId: string; userId: string } },
    @Param('id') id: string,
  ) {
    return this.usersService.remove(req.user.storeId, id, req.user.userId);
  }
}
