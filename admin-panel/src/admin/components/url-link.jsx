import { ValueGroup } from '@adminjs/design-system';
import { flat, useTranslation } from 'adminjs';
import React from 'react';

/**
 * Рендерит значение свойства как кликабельную ссылку: подсвечивается синим
 * и открывается в новой вкладке. Используется в list/show для URL-полей
 * (group_url, post_url, user_profile_url).
 *
 * В list заголовок колонки сам показывает название поля, а в show (детальный
 * просмотр) название рисуем сами через ValueGroup, иначе оно теряется.
 */
const UrlLink = ({ property, record, where }) => {
  const { translateProperty } = useTranslation();
  const value = flat.get(record.params, property.path);

  const link = value
    ? React.createElement(
        'a',
        {
          href:
            /^https?:\/\//i.test(value) || value.startsWith('//')
              ? value
              : `https://${value}`,
          target: '_blank',
          rel: 'noopener noreferrer',
          style: {
            color: '#3040D6',
            textDecoration: 'underline',
            wordBreak: 'break-all',
          },
          onClick: (e) => e.stopPropagation(),
        },
        value,
      )
    : React.createElement('span', null, '');

  if (where === 'show') {
    return React.createElement(
      ValueGroup,
      { label: translateProperty(property.label, property.resourceId) },
      link,
    );
  }

  return link;
};

export default UrlLink;
