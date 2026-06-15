import {
  BulkDeleteAction,
  DeleteAction,
  EditAction,
  ListAction,
  NewAction,
  SearchAction,
  ShowAction,
} from 'adminjs';
import { resource } from './resources.service.js';
import { ErrorsService } from '../errors/errors.service.js';
import { Injectable } from '@nestjs/common';

@Injectable()
export class ActionsService {
  constructor(
    private readonly errorsService: ErrorsService,
    // private readonly logger: AdminJSLogger,
  ) {}

  /**
   * Обернуть экшены ресурса
   * @description Экшен - действие над ресурсом
   *
   * @param resources
   */
  wrapResourcesActions(resources: resource[]) {
    resources.forEach((resource: resource) => {
      if (!resource.options.actions) {
        return resource;
      }
      resource = this.wrapResourceActions(resource);
    });
  }

  /**
   * Обернуть экшен ресурса
   * @description Экшены ресурса: "new, show, edit, delete, bulkDelete, new, list, search"
   *
   * @param resource
   */
  private wrapResourceActions(resource: resource) {
    const wrappedResource: resource = resource;
    const actions = resource.options.actions;

    for (const key in actions) {
      // Если ресурс задал собственный handler (в т.ч. для стандартного экшена,
      // например скоупленный по пользователю list) — берём его, иначе дефолтный.
      const baseHandler = actions[key]?.handler ?? this.getDefaultActionHandler(key);

      // кастомные action'ы без handler'а и без дефолта оставляем как есть
      if (!baseHandler) continue;

      wrappedResource.options.actions![key] = {
        ...actions[key],
        handler: this.errorsService.withErrorHandler(baseHandler),
      };
    }

    return wrappedResource;
  }

  // private getActionBefore(action: any) {
  //   const logRequest = this.logger.logRequest;

  //   return action.before?.length
  //     ? [...action.before, logRequest]
  //     : [logRequest];
  // }

  // private getActionAfter(action: any) {
  //   const logResponse = this.logger.logResponse;

  //   return action.after?.length
  //     ? [...action.after, logResponse]
  //     : [logResponse];
  // }

  private getDefaultActionHandler(key: string) {
    switch (key) {
      case 'new':
        return NewAction.handler;
      case 'show':
        return ShowAction.handler;
      case 'edit':
        return EditAction.handler;
      case 'delete':
        return DeleteAction.handler;
      case 'bulkDelete':
        return BulkDeleteAction.handler;
      case 'list':
        return ListAction.handler;
      case 'search':
        return SearchAction.handler;
      default:
        return null;
    }
  }
}
