import {
  Column,
  CreateDateColumn,
  Entity,
  Index,
  JoinColumn,
  ManyToOne,
  PrimaryGeneratedColumn,
  UpdateDateColumn,
} from 'typeorm';
import { User } from '../users/user.entity';
import { ShiftTemplate } from '../shifts/shift-template.entity';

@Entity('schedules')
@Index(['storeId', 'userId', 'workDate'], { unique: true })
export class Schedule {
  @PrimaryGeneratedColumn('uuid')
  id!: string;

  @Column()
  storeId!: string;

  @Column()
  userId!: string;

  @ManyToOne(() => User, { onDelete: 'CASCADE' })
  @JoinColumn({ name: 'userId' })
  user!: User;

  @Column({ type: 'date' })
  workDate!: string;

  @Column()
  shiftTemplateId!: string;

  @ManyToOne(() => ShiftTemplate, { onDelete: 'RESTRICT' })
  @JoinColumn({ name: 'shiftTemplateId' })
  shiftTemplate!: ShiftTemplate;

  @Column({ default: 'normal' })
  status!: string;

  @Column({ default: 1 })
  version!: number;

  @Column({ nullable: true })
  note?: string;

  @Column({ nullable: true })
  createdById?: string;

  @Column({ nullable: true })
  updatedById?: string;

  @CreateDateColumn()
  createdAt!: Date;

  @UpdateDateColumn()
  updatedAt!: Date;
}
