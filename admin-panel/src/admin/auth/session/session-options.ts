import type { SessionOptions } from 'express-session';
import type { Redis } from 'ioredis';
import RedisStore from 'connect-redis';

export function createSessionOptions(params: {
  redis: Redis;
  secret: string;
  cookieName: string;
  isProd: boolean;
}): SessionOptions {
  const ONE_HOUR_SECONDS = 60 * 60;
  const ONE_HOUR_MS = ONE_HOUR_SECONDS * 1000;

  return {
    name: params.cookieName,
    store: new RedisStore({
      client: params.redis,
      ttl: ONE_HOUR_SECONDS,
    }),
    resave: false,
    saveUninitialized: false,
    secret: params.secret,
    cookie: {
      httpOnly: true,
      sameSite: 'lax',
      secure: params.isProd,
      maxAge: ONE_HOUR_MS,
    },
  };
}
