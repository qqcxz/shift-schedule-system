import {
  Column,
  CreateDateColumn,
  Entity,
  JoinColumn,
  ManyToOne,
  PrimaryGeneratedColumn,
  UpdateDateColumn,
} from 'typeorm';
import { User } from '../users/user.entity';

export enum RequestType {
  LEAVE = 'leave',
  SWAP = 'swap',
  CHANGE = 'change',
}

export enum RequestStatus {
  PENDING = 'pending',
  APPROVED = 'approved',
  REJECTED = 'rejected',
  CANCELLED = 'cancelled',
}

@Entity('schedule_requests')
export class ScheduleRequest {
  @PrimaryGeneratedColumn('uuid')
  id!: string;

  @Column()
  storeId!: string;

  @Column({ type: 'varchar' })
  type!: RequestType;

  @Column({ type: 'varchar', default: RequestStatus.PENDING })
  status!: RequestStatus;

  @Column()
  requesterId!: string;

  @ManyToOne(() => User, { eager: true, onDelete: 'CASCADE' })
  @JoinColumn({ name: 'requesterId' })
  requester!: User;

  @Column({ nullable: true })
  targetUserId?: string;

  @ManyToOne(() => User, { eager: true, nullable: true, onDelete: 'SET NULL' })
  @JoinColumn({ name: 'targetUserId' })
  targetUser?: User | null;

  @Column({ type: 'date' })
  fromDate!: string;

  @Column({ type: 'date', nullable: true })
  toDate?: string;

  @Column({ nullable: true })
  fromShiftId?: string;

  @Column({ nullable: true })
  toShiftId?: string;

  @Column({ type: 'text', nullable: true })
  reason?: string;

  @Column({ type: 'text', nullable: true })
  reviewNote?: string;

  @Column({ nullable: true })
  approverId?: string;

  @Column({ type: 'datetime', nullable: true })
  reviewedAt?: Date;

  @CreateDateColumn()
  createdAt!: Date;

  @UpdateDateColumn()
  updatedAt!: Date;
}
