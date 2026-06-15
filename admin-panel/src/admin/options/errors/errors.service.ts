import { Injectable } from '@nestjs/common';
import { ActionContext, ActionRequest, ActionResponse } from 'adminjs';

@Injectable()
export class ErrorsService {
  constructor() {}
  withErrorHandler(handler: any) {
    return async (
      request: ActionRequest,
      response: ActionResponse,
      context: ActionContext,
    ) => {
      try {
        if (!handler) {
          throw new Error('Обработчик события не предоставлен');
        }
        return await handler(request, response, context);
      } catch (error) {
        return this.mapErrorToAdminJS(error, context);
      }
    };
  }

  private mapErrorToAdminJS(err: any, context: ActionContext) {
    // TODO: кастомные ошибки
    // if (err) {
    // }
    return {
      record: context?.resource?.build({}), // билдим ресурс для отсутствия базовой ошибки
      notice: {
        message: 'Внутренняя ошибка сервера',
        type: 'error',
      },
    };
  }
}
