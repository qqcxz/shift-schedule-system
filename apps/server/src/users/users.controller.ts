import { Controller, Get, Req, UseGuards } from '@nestjs/common';
import { UsersService } from './users.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

@Controller('users')
@UseGuards(JwtAuthGuard)
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Get()
  async list(@Req() req: { user: { storeId: string } }) {
    const users = await this.usersService.findByStore(req.user.storeId);
    return users.map((user) => this.usersService.sanitize(user));
  }
}
