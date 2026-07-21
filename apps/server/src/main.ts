import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
import { NestExpressApplication } from '@nestjs/platform-express';
import { NextFunction, Request, Response } from 'express';
import { existsSync } from 'fs';
import { join } from 'path';
import { AppModule } from './app.module';

function resolveCorsOrigin() {
  const raw = process.env.CORS_ORIGIN || 'http://localhost:5173';
  const list = raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

  // 微信开发者工具 / 真机调试常见本地来源，以及未带 Origin 的请求
  const defaults = [
    'http://localhost:5173',
    'http://127.0.0.1:5173',
    'http://localhost',
    'http://127.0.0.1',
  ];

  const merged = Array.from(new Set([...list, ...defaults]));

  // 允许无 Origin 请求（部分小程序运行时不会带 Origin）
  return (origin: string | undefined, callback: (err: Error | null, allow?: boolean | string) => void) => {
    if (!origin) {
      callback(null, true);
      return;
    }
    if (merged.includes(origin) || merged.includes('*')) {
      callback(null, origin);
      return;
    }
    // 开发期放行微信开发者工具本地模拟器域名
    if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/.test(origin)) {
      callback(null, origin);
      return;
    }
    callback(null, false);
  };
}

function resolveWebDist() {
  const candidates = [
    process.env.WEB_DIST,
    join(__dirname, '..', '..', 'web', 'dist'),
    join(__dirname, '..', 'public'),
    join(process.cwd(), 'public'),
    '/app/public',
  ].filter(Boolean) as string[];

  return candidates.find((dir) => existsSync(join(dir, 'index.html')));
}

async function bootstrap() {
  const app = await NestFactory.create<NestExpressApplication>(AppModule);
  app.setGlobalPrefix('api');
  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      transform: true,
      forbidNonWhitelisted: true,
    }),
  );

  app.enableCors({
    origin: resolveCorsOrigin(),
    credentials: true,
  });

  const webDist = resolveWebDist();
  if (webDist) {
    app.useStaticAssets(webDist, {
      index: false,
    });

    // SPA fallback：API / WebSocket 之外的 GET 请求回退到前端
    app.use((req: Request, res: Response, next: NextFunction) => {
      if (req.method !== 'GET' && req.method !== 'HEAD') {
        next();
        return;
      }

      const path = req.path || '';
      if (
        path.startsWith('/api') ||
        path.startsWith('/socket.io') ||
        path.includes('.')
      ) {
        next();
        return;
      }

      res.sendFile(join(webDist, 'index.html'), (err?: Error) => {
        if (err) next(err);
      });
    });

    // eslint-disable-next-line no-console
    console.log(`Serving web UI from ${webDist}`);
  }

  const port = Number(process.env.PORT || 3000);
  await app.listen(port, '0.0.0.0');
  // eslint-disable-next-line no-console
  console.log(`App running on http://0.0.0.0:${port}`);
  // eslint-disable-next-line no-console
  console.log(`API: http://0.0.0.0:${port}/api`);
}

bootstrap();
