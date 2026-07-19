import { IsIn, IsOptional, IsString } from 'class-validator';

export class ReviewRequestDto {
  @IsOptional()
  @IsIn(['approve', 'reject'])
  action?: 'approve' | 'reject';

  @IsOptional()
  @IsString()
  reviewNote?: string;
}
