import { Type } from 'class-transformer';
import {
  ArrayMinSize,
  IsArray,
  IsNumber,
  IsOptional,
  IsString,
  Matches,
  ValidateNested,
} from 'class-validator';

class ScheduleItemDto {
  @IsString()
  userId!: string;

  @Matches(/^\d{4}-\d{2}-\d{2}$/)
  workDate!: string;

  @IsString()
  shiftTemplateId!: string;

  @IsOptional()
  @IsString()
  status?: string;

  @IsOptional()
  @IsString()
  note?: string;

  @IsOptional()
  @IsNumber()
  version?: number;
}

export class SaveMonthScheduleDto {
  @Matches(/^\d{4}-\d{2}$/)
  month!: string;

  @IsArray()
  @ArrayMinSize(1)
  @ValidateNested({ each: true })
  @Type(() => ScheduleItemDto)
  items!: ScheduleItemDto[];
}
