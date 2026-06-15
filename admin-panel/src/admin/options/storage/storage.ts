import { Database, Resource } from '@adminjs/prisma';
import AdminJS from 'adminjs';

/**
 * Регистрирует различные адаптеры баз данных, написанные для AdminJS.
 * @description В текущем случае - prisma из пакета @adminjs/prisma
 */
export function setStorage() {
  AdminJS.registerAdapter({ Database, Resource });
}
