import { IsEnum, IsOptional, IsString, MaxLength, MinLength } from 'class-validator';
import { UserRole } from '../user.entity';

export class CreateUserDto {
  @IsString()
  @MinLength(2)
  @MaxLength(32)
  username!: string;

  @IsString()
  @MinLength(2)
  @MaxLength(32)
  displayName!: string;

  @IsString()
  @MinLength(6)
  @MaxLength(64)
  password!: string;

  @IsOptional()
  @IsEnum(UserRole)
  role?: UserRole;
}
