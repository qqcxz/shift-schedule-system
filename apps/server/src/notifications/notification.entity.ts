import { Column, CreateDateColumn, Entity, PrimaryGeneratedColumn } from 'typeorm';

@Entity('notifications')
export class Notification {
  @PrimaryGeneratedColumn('uuid')
  id!: string;

  @Column()
  storeId!: string;

  @Column()
  userId!: string;

  @Column()
  title!: string;

  @Column({ type: 'text' })
  content!: string;

  @Column({ default: false })
  isRead!: boolean;

  @Column({ nullable: true })
  relatedType?: string;

  @Column({ nullable: true })
  relatedId?: string;

  @CreateDateColumn()
  createdAt!: Date;
}
