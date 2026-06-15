import { Module } from '@nestjs/common';
import { AdminOptions } from './admin/admin.options.js';
import { AuthModule } from '../../../auth/auth.module.js';
import { PrismaModule } from '../../../../../prisma/prisma.module.js';

@Module({
  imports: [AuthModule, PrismaModule],
  exports: [AdminOptions],
  providers: [AdminOptions],
})
export class OptionsModule {}
