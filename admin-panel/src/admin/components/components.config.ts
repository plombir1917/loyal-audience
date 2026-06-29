import { ComponentLoader } from 'adminjs';
import path from 'path';

export const componentLoader = new ComponentLoader();

export const Components = {
  UrlLink: componentLoader.add(
    'UrlLink',
    path.join(__dirname, 'url-link'),
  ),
  ExportExcel: componentLoader.add(
    'ExportExcel',
    path.join(__dirname, 'export-excel'),
  ),
  Dashboard: componentLoader.add(
    'Dashboard',
    path.join(__dirname, 'dashboard'),
  ),
};
