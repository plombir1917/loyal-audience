import {
  ActionContext,
  ActionRequest,
  Before,
  ForbiddenError,
  RecordJSON,
} from 'adminjs';

interface SearchResponse {
  records?: RecordJSON[];
}

/**
 * before-хук для `show`: пускает к записи только её владельца.
 *
 * @description Если запись не принадлежит авторизованному пользователю
 * (или вовсе не существует) — кидаем 403, не раскрывая факт её существования.
 *
 * @param owns проверка владения: (userId, recordId) => принадлежит ли запись
 */
export function ownershipShowGuard(
  owns: (userId: string, recordId: string) => Promise<boolean>,
): Before {
  return async (request: ActionRequest, context: ActionContext) => {
    const userId = context.currentAdmin?.id;
    const recordId = request.params?.recordId;
    if (userId && recordId && !(await owns(userId, recordId))) {
      throw new ForbiddenError('Нет доступа к этой записи');
    }
    return request;
  };
}

/**
 * after-хук для `search`: оставляет в выдаче только записи пользователя.
 *
 * @description search используется в автокомплите справочников и возвращает
 * сериализованные записи (RecordJSON) — фильтруем их по владельцу.
 *
 * @param keep фильтр: (userId, records) => только принадлежащие пользователю
 */
export function ownershipSearchFilter(
  keep: (userId: string, records: RecordJSON[]) => Promise<RecordJSON[]>,
) {
  return async (
    response: SearchResponse,
    _request: ActionRequest,
    context: ActionContext,
  ): Promise<SearchResponse> => {
    const userId = context.currentAdmin?.id;
    if (!userId) {
      return { ...response, records: [] };
    }
    return { ...response, records: await keep(userId, response.records ?? []) };
  };
}
