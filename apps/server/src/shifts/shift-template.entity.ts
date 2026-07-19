import { Column, CreateDateColumn, Entity, PrimaryGeneratedColumn, UpdateDateColumn } from 'typeorm';

@Entity('shift_templates')
export class ShiftTemplate {
  @PrimaryGeneratedColumn('uuid')
  id!: string;

  @Column()
  storeId!: string;

  @Column()
  name!: string;

  @Column()
  code!: string;

  @Column()
  startTime!: string;

  @Column()
  endTime!: string;

  @Column({ default: '#409EFF' })
  color!: string;

  @Column({ default: 0 })
  sortOrder!: number;

  @Column({ default: false })
  isOff!: boolean;

  @CreateDateColumn()
  createdAt!: Date;

  @UpdateDateColumn()
  updatedAt!: Date;
}
