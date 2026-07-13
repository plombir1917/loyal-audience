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
  RecalculateStats: componentLoader.add(
    'RecalculateStats',
    path.join(__dirname, 'recalculate-stats'),
  ),
  Dashboard: componentLoader.add(
    'Dashboard',
    path.join(__dirname, 'dashboard'),
  ),
};
