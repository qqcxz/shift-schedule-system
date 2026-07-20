import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
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

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
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

  const port = Number(process.env.PORT || 3000);
  await app.listen(port, '0.0.0.0');
  // eslint-disable-next-line no-console
  console.log(`API running on http://0.0.0.0:${port}/api`);
}

bootstrap();
