import {
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Post,
  Req,
  UseGuards,
} from '@nestjs/common';
import { NotificationsService } from './notifications.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';

@Controller('notifications')
@UseGuards(JwtAuthGuard)
export class NotificationsController {
  constructor(private readonly notificationsService: NotificationsService) {}

  @Get()
  list(@Req() req: { user: { userId: string } }) {
    return this.notificationsService.list(req.user.userId);
  }

  @Post('read-all')
  markAll(@Req() req: { user: { userId: string } }) {
    return this.notificationsService.markAllRead(req.user.userId);
  }

  @Post('clear')
  clear(
    @Req() req: { user: { userId: string } },
    @Body() body: { mode?: 'read' | 'all' },
  ) {
    return this.notificationsService.clear(req.user.userId, body?.mode || 'read');
  }

  @Post(':id/read')
  markRead(@Req() req: { user: { userId: string } }, @Param('id') id: string) {
    return this.notificationsService.markRead(req.user.userId, id);
  }

  @Delete(':id')
  remove(@Req() req: { user: { userId: string } }, @Param('id') id: string) {
    return this.notificationsService.remove(req.user.userId, id);
  }
}
