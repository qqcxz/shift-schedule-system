import {
  Body,
  Controller,
  Get,
  Param,
  Post,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { RequestsService } from './requests.service';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { Roles, RolesGuard } from '../common/roles.decorator';
import { UserRole } from '../users/user.entity';
import { CreateRequestDto } from './dto/create-request.dto';
import { ReviewRequestDto } from './dto/review-request.dto';
import { RequestStatus } from './schedule-request.entity';

@Controller('requests')
@UseGuards(JwtAuthGuard, RolesGuard)
export class RequestsController {
  constructor(private readonly requestsService: RequestsService) {}

  @Get()
  list(
    @Req() req: { user: { storeId: string; userId: string; role: UserRole } },
    @Query('status') status?: RequestStatus,
  ) {
    return this.requestsService.list(req.user.storeId, req.user.userId, req.user.role, status);
  }

  @Post()
  create(
    @Req() req: { user: { storeId: string; userId: string; role: UserRole } },
    @Body() dto: CreateRequestDto,
  ) {
    return this.requestsService.create(req.user.storeId, req.user.userId, req.user.role, dto);
  }

  @Post(':id/approve')
  @Roles(UserRole.MANAGER)
  approve(
    @Req() req: { user: { storeId: string; userId: string } },
    @Param('id') id: string,
    @Body() dto: ReviewRequestDto,
  ) {
    return this.requestsService.review(req.user.storeId, req.user.userId, id, {
      action: 'approve',
      reviewNote: dto.reviewNote,
    });
  }

  @Post(':id/reject')
  @Roles(UserRole.MANAGER)
  reject(
    @Req() req: { user: { storeId: string; userId: string } },
    @Param('id') id: string,
    @Body() dto: ReviewRequestDto,
  ) {
    return this.requestsService.review(req.user.storeId, req.user.userId, id, {
      action: 'reject',
      reviewNote: dto.reviewNote,
    });
  }
}
