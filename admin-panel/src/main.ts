import { HttpAdapterHost, NestFactory } from '@nestjs/core';
import { NestExpressApplication } from '@nestjs/platform-express';
import { BadRequestException, ValidationPipe } from '@nestjs/common';
import { AppModule } from './app.module';
import { join } from 'path';
import * as express from 'express';
import helmet from 'helmet';
import compression from 'compression';
import { PrismaClientExceptionFilter } from 'prisma/prisma-client-exception.filter';

// Prisma возвращает user_vk_id как BigInt, а JSON.stringify по нему падает.
// Сериализуем BigInt в строку, чтобы AdminJS мог отрисовать записи.
(BigInt.prototype as unknown as { toJSON: () => string }).toJSON = function () {
  return this.toString();
};

async function bootstrap() {
  const app = await NestFactory.create<NestExpressApplication>(AppModule);

  // Security headers. CSP is left disabled because the landing relies on inline
  // styles/scripts and AdminJS injects its own assets; the remaining headers
  // (HSTS, nosniff, frameguard, referrer-policy) apply site-wide. CORP is set
  // to cross-origin so social scrapers / CDNs can fetch og-image.png & icons.
  app.use(
    helmet({
      contentSecurityPolicy: false,
      crossOriginEmbedderPolicy: false,
      crossOriginResourcePolicy: { policy: 'cross-origin' },
      // VK ID при alternative-login открывает попап и общается через
      // window.opener — дефолтный COOP 'same-origin' это рвёт. Разрешаем
      // попапы, сохраняя изоляцию от чужих окон.
      crossOriginOpenerPolicy: { policy: 'same-origin-allow-popups' },
      referrerPolicy: { policy: 'strict-origin-when-cross-origin' },
    }),
  );

  // gzip/brotli-style response compression (HTML, JS, CSS, JSON, SVG).
  app.use(compression());

  app.use(
    '/',
    express.static(join(__dirname, 'frontend/public'), {
      setHeaders: (res, filePath) => {
        const file = filePath.replace(/\\/g, '/');
        if (file.endsWith('.webmanifest')) {
          res.setHeader(
            'Content-Type',
            'application/manifest+json; charset=utf-8',
          );
        }
        if (file.endsWith('.ico')) {
          // Match the type declared in the <link rel="icon"> tags
          // (image/x-icon). serve-static defaults to image/vnd.microsoft.icon,
          // and Yandex's favicon checker wants the served Content-Type to match
          // the link's `type` attribute exactly.
          res.setHeader('Content-Type', 'image/x-icon');
        }
        if (file.endsWith('/sw.js')) {
          // The service worker must always be revalidated, and is allowed to
          // control the whole origin.
          res.setHeader('Cache-Control', 'no-cache');
          res.setHeader('Service-Worker-Allowed', '/');
        } else if (/\.(png|ico|jpg|jpeg|svg|webp|woff2?)$/.test(file)) {
          res.setHeader('Cache-Control', 'public, max-age=604800'); // 7d for assets
        } else if (/\.(exe|zip|AppImage|deb)$/i.test(file)) {
          // Desktop installers: force a binary download with an explicit
          // filename. Correct Content-Type/Disposition avoids the browser
          // sniffing the file and is one less reason for download warnings.
          res.setHeader('Content-Type', 'application/octet-stream');
          res.setHeader(
            'Content-Disposition',
            `attachment; filename="${file.split('/').pop()}"`,
          );
        }
      },
    }),
  );

  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      exceptionFactory: (errors) => {
        const fieldErrors: Record<string, string> = {};
        for (const err of errors) {
          fieldErrors[err.property] =
            Object.values(err.constraints ?? {})[0] ?? 'Ошибка';
        }
        return new BadRequestException({ statusCode: 400, fieldErrors });
      },
    }),
  );

  const { httpAdapter } = app.get(HttpAdapterHost);
  app.useGlobalFilters(new PrismaClientExceptionFilter(httpAdapter));

  await app.listen(process.env.PORT ?? 3000);
}
bootstrap();
