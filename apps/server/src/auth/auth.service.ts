import { Injectable, UnauthorizedException } from '@nestjs/common';
import { JwtService } from '@nestjs/jwt';
import * as bcrypt from 'bcryptjs';
import { UsersService } from '../users/users.service';
import { LoginDto } from './dto/login.dto';

@Injectable()
export class AuthService {
  constructor(
    private readonly usersService: UsersService,
    private readonly jwtService: JwtService,
  ) {}

  async login(dto: LoginDto) {
    const user = await this.usersService.findByUsername(dto.username);
    if (!user || !user.isActive) {
      throw new UnauthorizedException('用户名或密码错误');
    }

    const matched = await bcrypt.compare(dto.password, user.passwordHash);
    if (!matched) {
      throw new UnauthorizedException('用户名或密码错误');
    }

    const token = await this.jwtService.signAsync({
      sub: user.id,
      role: user.role,
      storeId: user.storeId,
      username: user.username,
    });

    return {
      accessToken: token,
      user: this.usersService.sanitize(user),
    };
  }
}
