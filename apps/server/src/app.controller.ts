import { Controller, Get } from '@nestjs/common';

@Controller()
export class AppController {
  @Get()
  root() {
    return {
      ok: true,
      service: 'shift-schedule-api',
      message: '排班系统后端运行正常',
      docs: {
        health: 'GET /api/health',
        login: 'POST /api/auth/login',
        me: 'GET /api/auth/me',
        schedules: 'GET /api/schedules?month=YYYY-MM',
        requests: 'GET /api/requests',
        notifications: 'GET /api/notifications',
      },
      frontend: 'http://localhost:5173',
      time: new Date().toISOString(),
    };
  }

  @Get('health')
  health() {
    return {
      ok: true,
      service: 'shift-schedule-api',
      time: new Date().toISOString(),
    };
  }
}
