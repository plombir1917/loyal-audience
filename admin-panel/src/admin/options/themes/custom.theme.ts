import { ThemeConfig } from 'adminjs';

/**
 * Кастомная тема для модуля AdminJs.
 * @description Поля со значением `light` оставить по умолчанию, т.к. мы переопределяем дефолтную тему. Иначе сыпятся ошибки в консоль.
 */
export const customTheme: ThemeConfig = {
  id: 'light',
  name: 'light',
  overrides: {},
};
