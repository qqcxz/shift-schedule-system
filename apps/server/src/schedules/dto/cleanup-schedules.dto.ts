import { Type } from 'class-transformer';
import { IsInt, IsOptional, Matches, Max, Min } from 'class-validator';

export class CleanupSchedulesDto {
  /**
   * 删除指定月份之前的全部排班（不含该月）。
   * 例如 beforeMonth=2026-01 表示删除 2025-12 及更早的数据。
   */
  @IsOptional()
  @Matches(/^\d{4}-\d{2}$/)
  beforeMonth?: string;

  /**
   * 仅保留最近 N 个月（含当前月）的排班，更早的全部删除。
   * 例如 keepMonths=6，当前是 2026-07，则保留 2026-02 ~ 2026-07。
   */
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(120)
  keepMonths?: number;
}
