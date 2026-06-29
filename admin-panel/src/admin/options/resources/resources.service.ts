import { getModelByName } from '@adminjs/prisma';
import { Injectable } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../../../../prisma/prisma.service.js';
import { AdminOptions } from './options/admin/admin.options.js';
import { FeatureType, ResourceOptions, ResourceWithOptions } from 'adminjs';
import { ActionsService } from './actions.service.js';
import { ExportService } from './export/export.service.js';
import { Components } from '../../components/components.config.js';

export interface resource {
  model?: Prisma.ModelName;
  options: ResourceOptions;
  features?: Array<FeatureType>;
}

// Данные собирает Go-сервис (vk-loyal-users-parser); в админке они доступны
// только для чтения, поэтому create/edit/delete отключены.
const VK_NAVIGATION = 'ВКонтакте';

function readOnly(icon: string): ResourceOptions {
  return {
    navigation: { name: VK_NAVIGATION, icon },
    actions: {
      new: { isAccessible: false },
      edit: { isAccessible: false },
      delete: { isAccessible: false },
      bulkDelete: { isAccessible: false },
    },
  };
}

// Делает URL-поля кликабельными ссылками (синие, открываются в новой вкладке)
// в списке и на странице записи.
function withUrlLinks(
  options: ResourceOptions,
  urlProperties: string[],
): ResourceOptions {
  const properties = { ...(options.properties ?? {}) };
  for (const prop of urlProperties) {
    properties[prop] = {
      ...(properties[prop] ?? {}),
      components: {
        ...(properties[prop]?.components ?? {}),
        list: Components.UrlLink,
        show: Components.UrlLink,
      },
    };
  }
  return { ...options, properties };
}

@Injectable()
export class ResourceService {
  constructor(
    private readonly prismaService: PrismaService,
    private readonly actionsService: ActionsService,
    private readonly adminOptions: AdminOptions,
    private readonly exportService: ExportService,
  ) {}

  // Добавляет ресурсу кнопку «Excel» (выгрузка всей таблицы в файл .xlsx).
  // Это resource-action — кнопка появляется в тулбаре списка сущности.
  private withExcelExport(
    options: ResourceOptions,
    resourceId: string,
  ): ResourceOptions {
    return {
      ...options,
      actions: {
        ...(options.actions ?? {}),
        exportExcel: {
          actionType: 'resource',
          icon: 'Download',
          // Подпись кнопки берётся из локали: actions.exportExcel.
          component: Components.ExportExcel,
          handler: async () => this.exportService.exportResource(resourceId),
        },
      },
    };
  }

  /**
   * Ресурс - сущность программы из БД
   */
  private resources(): resource[] {
    return [
      {
        model: Prisma.ModelName.admin,
        options: this.adminOptions.get(),
      },
      {
        model: Prisma.ModelName.community,
        options: this.withExcelExport(
          withUrlLinks(readOnly('Users'), ['group_url']),
          'community',
        ),
      },
      {
        model: Prisma.ModelName.post,
        options: this.withExcelExport(
          withUrlLinks(readOnly('FileText'), ['post_url']),
          'post',
        ),
      },
      {
        model: Prisma.ModelName.comment,
        options: this.withExcelExport(readOnly('MessageSquare'), 'comment'),
      },
      {
        model: Prisma.ModelName.like,
        options: this.withExcelExport(readOnly('Heart'), 'like'),
      },
      {
        model: Prisma.ModelName.user,
        options: this.withExcelExport(
          withUrlLinks(readOnly('User'), ['user_profile_url']),
          'user',
        ),
      },
    ];
  }

  getResourcesWithOptions() {
    const resourcesWithOptions: ResourceWithOptions[] = [];
    this.resources().forEach((resource) => {
      resourcesWithOptions.push(this.setResourceOptions(resource));
    });
    this.actionsService.wrapResourcesActions(resourcesWithOptions);
    return resourcesWithOptions;
  }

  private setResourceOptions(resource: resource): ResourceWithOptions {
    return {
      resource: {
        model: getModelByName(resource.model!),
        client: this.prismaService,
      },
      options: resource.options,
      features: resource.features,
    };
  }
}
