import { BaseAuthProvider, CurrentAdmin } from 'adminjs';

/**
 * Абсракция для обёртывания параметров объекта авторизации модуля AdminJs
 */
export interface AdminJsAuth {
  authenticate?: (
    email: string,
    password: string,
    ctx?: any,
  ) => Promise<CurrentAdmin | null>;
  cookiePassword: string;
  cookieName: string;
  provider?: BaseAuthProvider;
}
