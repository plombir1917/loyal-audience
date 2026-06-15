import {
  ActionContext,
  ActionRequest,
  ActionResponse,
  flat,
} from 'adminjs';
import { UserScopedQuery } from '../../../../../prisma/prisma.service.js';

const PER_PAGE_LIMIT = 500;
const DEFAULT_PER_PAGE = 10;

export interface ScopedPage {
  rows: Record<string, unknown>[];
  total: number;
}

/** Выборка из БД, уже скоупленная по user_id. */
export type ScopedFetcher = (
  userId: string,
  query: UserScopedQuery,
) => Promise<ScopedPage>;

/**
 * Собрать list-handler AdminJS, который отдаёт только записи,
 * принадлежащие авторизованному пользователю (`currentAdmin`).
 *
 * @description attention нельзя отфильтровать штатным фильтром AdminJS
 * (у него нет собственного user_id), поэтому выборку делаем напрямую через
 * Prisma, сохраняя пагинацию и сортировку из запроса.
 *
 * @param fetch       функция выборки, скоупленная по user_id
 * @param sortable    колонки, по которым разрешена сортировка
 * @param defaultSort колонка сортировки по умолчанию
 */
export function scopedListHandler(
  fetch: ScopedFetcher,
  sortable: string[],
  defaultSort: string,
) {
  return async (
    request: ActionRequest,
    _response: ActionResponse,
    context: ActionContext,
  ) => {
    const { resource, currentAdmin } = context;
    const userId = currentAdmin?.id;

    // Без авторизованного пользователя возвращаем пустой список, а не чужие данные.
    if (!userId) {
      return { records: [], meta: { total: 0, perPage: DEFAULT_PER_PAGE, page: 1 } };
    }

    const query = flat.unflatten(request.query ?? {}) as Record<string, any>;
    const page = Number(query.page) || 1;
    const perPage = Math.min(
      Number(query.perPage) || DEFAULT_PER_PAGE,
      PER_PAGE_LIMIT,
    );
    const sortBy = sortable.includes(query.sortBy) ? query.sortBy : defaultSort;
    const direction = query.direction === 'asc' ? 'asc' : 'desc';

    const { rows, total } = await fetch(userId, {
      skip: (page - 1) * perPage,
      take: perPage,
      orderBy: { [sortBy]: direction },
    });

    return {
      meta: { total, perPage, page, direction, sortBy },
      records: rows.map((row) => resource.build(row).toJSON(currentAdmin)),
    };
  };
}
