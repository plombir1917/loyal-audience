import { Module } from '@nestjs/common';
import { AdminJSModule } from './admin/admin.module';
import { RedisModule } from 'redis/redis.module';

@Module({
  imports: [AdminJSModule, RedisModule],
  controllers: [],
})
export class AppModule {}
