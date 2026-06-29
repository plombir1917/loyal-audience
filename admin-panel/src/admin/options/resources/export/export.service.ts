import { Injectable } from '@nestjs/common';
import ExcelJS from 'exceljs';
import { PrismaService } from '../../../../../prisma/prisma.service.js';

const XLSX_MIME =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';

interface Column {
  key: string;
  header: string;
}

interface ExportEntity {
  // id совпадает с именем модели Prisma / resourceId в AdminJS.
  id: string;
  // Название листа в книге Excel (он же — человекочитаемый заголовок).
  sheet: string;
  columns: Column[];
  fetch: () => Promise<Record<string, unknown>[]>;
}

export interface ExcelFile {
  base64: string;
  filename: string;
  mimeType: string;
}

@Injectable()
export class ExportService {
  constructor(private readonly prisma: PrismaService) {}

  /**
   * Сущности БД, доступные для выгрузки в Excel (всё, кроме администраторов).
   * Колонки заданы явно — чтобы порядок и заголовки были стабильны даже для
   * пустой таблицы.
   */
  private entities(): ExportEntity[] {
    return [
      {
        id: 'community',
        sheet: 'Сообщества',
        columns: [
          { key: 'group_id', header: 'ID группы' },
          { key: 'group_name', header: 'Название' },
          { key: 'group_url', header: 'Ссылка' },
          { key: 'group_description', header: 'Описание' },
          { key: 'group_subscribers', header: 'Подписчиков' },
          { key: 'region', header: 'Регион' },
          { key: 'city', header: 'Город' },
        ],
        fetch: () => this.prisma.community.findMany(),
      },
      {
        id: 'post',
        sheet: 'Посты',
        columns: [
          { key: 'post_id', header: 'ID поста' },
          { key: 'group_id', header: 'ID группы' },
          { key: 'post_text', header: 'Текст' },
          { key: 'post_date', header: 'Дата' },
          { key: 'post_url', header: 'Ссылка' },
        ],
        fetch: () => this.prisma.post.findMany(),
      },
      {
        id: 'comment',
        sheet: 'Комментарии',
        columns: [
          { key: 'comment_id', header: 'ID комментария' },
          { key: 'post_id', header: 'ID поста' },
          { key: 'user_id', header: 'ID пользователя' },
          { key: 'comment_text', header: 'Текст' },
          { key: 'comment_date', header: 'Дата' },
          { key: 'sentiment', header: 'Тональность' },
        ],
        fetch: () => this.prisma.comment.findMany(),
      },
      {
        id: 'like',
        sheet: 'Лайки',
        columns: [
          { key: 'like_id', header: 'ID лайка' },
          { key: 'post_id', header: 'ID поста' },
          { key: 'user_id', header: 'ID пользователя' },
        ],
        fetch: () => this.prisma.like.findMany(),
      },
      {
        id: 'user',
        sheet: 'Пользователи',
        columns: [
          { key: 'user_id', header: 'ID пользователя' },
          { key: 'user_vk_id', header: 'VK ID' },
          { key: 'user_profile_url', header: 'Профиль' },
          { key: 'segment', header: 'Сегмент' },
        ],
        fetch: () => this.prisma.user.findMany(),
      },
    ];
  }

  /** Выгрузка одной сущности — один лист в книге. */
  async exportResource(resourceId: string): Promise<ExcelFile> {
    const entity = this.entities().find((e) => e.id === resourceId);
    if (!entity) {
      throw new Error(`Экспорт недоступен для ресурса «${resourceId}»`);
    }
    const workbook = new ExcelJS.Workbook();
    await this.addSheet(workbook, entity);
    return this.toFile(workbook, `${entity.id}.xlsx`);
  }

  /** Выгрузка всей БД — каждая сущность на отдельном листе. */
  async exportAll(): Promise<ExcelFile> {
    const workbook = new ExcelJS.Workbook();
    for (const entity of this.entities()) {
      await this.addSheet(workbook, entity);
    }
    return this.toFile(workbook, 'database.xlsx');
  }

  private async addSheet(
    workbook: ExcelJS.Workbook,
    entity: ExportEntity,
  ): Promise<void> {
    const rows = await entity.fetch();
    const sheet = workbook.addWorksheet(entity.sheet);
    sheet.columns = entity.columns.map((c) => ({
      header: c.header,
      key: c.key,
      width: 28,
    }));
    rows.forEach((row) => {
      const normalized: Record<string, unknown> = {};
      entity.columns.forEach((c) => {
        normalized[c.key] = this.normalize(row[c.key]);
      });
      sheet.addRow(normalized);
    });
    sheet.getRow(1).font = { bold: true };
  }

  private normalize(value: unknown): string | number | Date {
    if (value === null || value === undefined) return '';
    if (typeof value === 'bigint') return value.toString();
    if (value instanceof Date) return value;
    if (typeof value === 'object') return JSON.stringify(value);
    if (typeof value === 'number') return value;
    return String(value);
  }

  private async toFile(
    workbook: ExcelJS.Workbook,
    filename: string,
  ): Promise<ExcelFile> {
    const buffer = await workbook.xlsx.writeBuffer();
    return {
      base64: Buffer.from(buffer).toString('base64'),
      filename,
      mimeType: XLSX_MIME,
    };
  }
}
