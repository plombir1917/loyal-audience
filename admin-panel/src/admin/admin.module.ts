import { Module } from '@nestjs/common';
import { AdminJSService } from './admin.service.js';
import { AuthModule } from './auth/auth.module.js';
import { ResourceService } from './options/resources/resources.service.js';
import { ActionsService } from './options/resources/actions.service.js';
import { ErrorsService } from './options/errors/errors.service.js';
import { ExportService } from './options/resources/export/export.service.js';
import { StatsService } from './options/resources/stats/stats.service.js';
import { OptionsModule } from './options/resources/options/options.module.js';
import { PrismaModule } from '../../prisma/prisma.module.js';
import { RedisModule } from '@nestjs-modules/ioredis';

@Module({
  imports: [
    PrismaModule,
    // UserModule,
    AuthModule,
    OptionsModule,
    import('@adminjs/nestjs').then(({ AdminModule }) =>
      AdminModule.createAdminAsync({
        useFactory: (adminService: AdminJSService) =>
          adminService.initAdminJsConfig(),
        imports: [AdminJSModule],
        inject: [AdminJSService],
      }),
    ),
  ],
  providers: [
    AdminJSService,
    ResourceService,
    ActionsService,
    ErrorsService,
    ExportService,
    StatsService,
  ],
  exports: [AdminJSService],
})
export class AdminJSModule {}
