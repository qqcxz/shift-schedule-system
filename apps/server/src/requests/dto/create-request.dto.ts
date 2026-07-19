import { IsEnum, IsOptional, IsString, Matches } from 'class-validator';
import { RequestType } from '../schedule-request.entity';

export class CreateRequestDto {
  @IsEnum(RequestType)
  type!: RequestType;

  @Matches(/^\d{4}-\d{2}-\d{2}$/)
  fromDate!: string;

  @IsOptional()
  @Matches(/^\d{4}-\d{2}-\d{2}$/)
  toDate?: string;

  @IsOptional()
  @IsString()
  targetUserId?: string;

  @IsOptional()
  @IsString()
  fromShiftId?: string;

  @IsOptional()
  @IsString()
  toShiftId?: string;

  @IsOptional()
  @IsString()
  reason?: string;
}
