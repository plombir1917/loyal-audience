import { Injectable } from '@nestjs/common';
import { AuthService } from './auth/auth.service.js';
import { AdminJSOptions, Locale } from 'adminjs';
import { componentLoader, Components } from './components/components.config.js';
import { customTheme } from './options/themes/custom.theme.js';
import { SessionOptions } from 'express-session';
import { ResourceService } from './options/resources/resources.service.js';
import { ExportService } from './options/resources/export/export.service.js';
import { en } from './options/locales/en.locale.js';
import { AdminJsAuth } from './options/interfaces/auth.interface.js';
import { AdminJsBranding } from './options/interfaces/branding.interface.js';
import { InjectRedis } from '@nestjs-modules/ioredis';
import type { Redis } from 'ioredis';
import { createSessionOptions } from './auth/session/session-options.js';
import { setStorage } from './options/storage/storage.js';
import path from 'path';

@Injectable()
export class AdminJSService {
  constructor(
    @InjectRedis() private readonly redis: Redis,
    private readonly resourceServie: ResourceService,
    private readonly authService: AuthService,
    private readonly exportService: ExportService,
  ) {}

  /**
   * Инициализировать конфиг AdminJs модуля
   *
   * @description Возвращает объект с вложенными методами
   */
  initAdminJsConfig() {
    setStorage();
    return {
      shouldBeInitialized: true,

      adminJsOptions: this.getAdminJsOptions(),
      auth: this.getAdminJsAuth(),
      sessionOptions: this.getAdminJsSessionOptions(),
    };
  }

  /**
   * Получить основные параметры модуля AdminJs.
   *
   * @returns AdminJSOptions
   */
  getAdminJsOptions(): AdminJSOptions {
    return {
      componentLoader,
      defaultTheme: 'light',
      availableThemes: [customTheme],
      assets: {
        // styles: ['/style.css'],
        // vkid-auth.js должен идти ПЕРЕД admin-vkid.js (тот использует window.VkAuth).
      },
      branding: this.getBranding(),
      rootPath: '/admin',
      resources: this.getResources(),
      dashboard: this.getDashboard(),
      locale: this.getLocale(),
    };
  }

  /**
   * Дашборд: кастомный компонент с кнопкой выгрузки всей БД в Excel.
   *
   * @description Обычный вызов (без query) ничего не считает. Когда фронтенд
   * запрашивает getDashboard с ?format=xlsx — отдаём книгу со всеми таблицами,
   * каждая сущность на отдельном листе.
   */
  getDashboard() {
    return {
      component: Components.Dashboard,
      handler: async (request: { query?: Record<string, unknown> }) => {
        if (request?.query?.format === 'xlsx') {
          return this.exportService.exportAll();
        }
        return {};
      },
    };
  }

  /**
   * Получить параметры сессии модуля AdminJs.
   *
   * @description Возвращает объект SessionOptions из express-session
   */
  getAdminJsSessionOptions(): SessionOptions {
    // if (!process.env.SECRET) {
    //   throw new Error('No secret value in .env');
    // }

    const cookieName = 'auth';
    const isProd = process.env.NODE_ENV === 'prod';

    return createSessionOptions({
      redis: this.redis,
      secret: process.env.SECRET!,
      cookieName,
      isProd,
    });
  }

  /**
   * Получить параметры авторизации модуля AdminJs.
   *
   * @description Возвращает реализацию метода authenticate, параметры cookie.
   */
  getAdminJsAuth(): AdminJsAuth {
    // if (!process.env.SECRET) {
    //   throw new Error('No secret value in .env');
    // }
    return {
      // AdminJS вызывает authenticate(email, password, context), где
      // context = { req, res }, а req.fields (express-formidable) содержит все
      // поля формы. VK-вход сабмитит скрытую форму с полем vkAccessToken — при
      // его наличии идём по ветке VK ID, иначе — штатный вход email/пароль.
      authenticate: async (email, password, context?: { req?: any }) => {
        return await this.authService.login(email, password);
      },
      cookieName: 'auth',
      cookiePassword: process.env.SECRET!,
    };
  }

  /**
   * Получить брэндирование модуля AdminJs.
   *
   * @description Возвращает данные о компании, которая использует продукт.
   *
   */
  getBranding(): AdminJsBranding {
    return {
      companyName: 'ПАРСЕР | Админ-панель',
      withMadeWithLove: false,
      logo: '/logo.png',
      favicon: '/favicon.ico',
    };
  }

  /**
   * Получить локализацию.
   * @description Возвращает язык и локализацию. `en` установлен специально, скрывает за собой `ru`, во избежании ошибок.
   */
  getLocale(): Locale {
    return {
      localeDetection: true,
      language: 'en',
      translations: {
        en,
      },
    };
  }

  /**
   * Получить кастомный компонент дашборда.
   */
  // getDashboard() {
  //   return this.dashboardService.getDashboard();
  // }

  /**
   * Получить все ресурсы.
   * @description Все модели данных, определённые в prisma.
   */
  getResources() {
    return this.resourceServie.getResourcesWithOptions();
  }
}
