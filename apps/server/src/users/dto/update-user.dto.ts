import { IsBoolean, IsEnum, IsOptional, IsString, MaxLength, MinLength } from 'class-validator';
import { UserRole } from '../user.entity';

export class UpdateUserDto {
  @IsOptional()
  @IsString()
  @MinLength(2)
  @MaxLength(32)
  username?: string;

  @IsOptional()
  @IsString()
  @MinLength(2)
  @MaxLength(32)
  displayName?: string;

  @IsOptional()
  @IsString()
  @MinLength(6)
  @MaxLength(64)
  password?: string;

  @IsOptional()
  @IsEnum(UserRole)
  role?: UserRole;

  @IsOptional()
  @IsBoolean()
  isActive?: boolean;
}
