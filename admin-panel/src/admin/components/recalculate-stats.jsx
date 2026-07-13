import { Box, Button, Loader, MessageBox, Text } from '@adminjs/design-system';
import { ApiClient } from 'adminjs';
import React, { useCallback, useEffect, useState } from 'react';

const api = new ApiClient();

// Путь к списку ресурса = текущий URL без хвоста /actions/<name>.
const listPath = () =>
  window.location.pathname.replace(/\/actions\/[^/]+\/?$/, '');

/**
 * Страница resource-action «Рассчитать»: при открытии пересчитывает статистику
 * на сервере из данных, уже лежащих в БД (вне зависимости от парсера), затем
 * возвращает к списку — таблица показывает свежие данные.
 */
const RecalculateStats = ({ resource, action }) => {
  const [status, setStatus] = useState('loading'); // loading | done | error

  const run = useCallback(() => {
    setStatus('loading');
    api
      .resourceAction({ resourceId: resource.id, actionName: action.name })
      .then(() => setStatus('done'))
      .catch(() => setStatus('error'));
  }, [resource.id, action.name]);

  useEffect(() => {
    run();
  }, [run]);

  // После успешного расчёта возвращаемся к списку со свежими данными.
  useEffect(() => {
    if (status !== 'done') return undefined;
    const timer = setTimeout(() => window.location.assign(listPath()), 1200);
    return () => clearTimeout(timer);
  }, [status]);

  return (
    <Box variant="grey">
      <Box variant="white">
        {status === 'loading' && (
          <Box flex flexDirection="column" alignItems="center" p="xxl">
            <Loader />
            <Text mt="lg">Рассчитываем статистику…</Text>
          </Box>
        )}

        {status === 'done' && (
          <Box p="xl">
            <MessageBox
              variant="success"
              message="Статистика рассчитана"
              data-testid="recalculate-success"
            >
              Данные обновлены. Открываем таблицу…
            </MessageBox>
            <Button
              mt="lg"
              variant="primary"
              onClick={() => window.location.assign(listPath())}
            >
              Открыть таблицу
            </Button>
          </Box>
        )}

        {status === 'error' && (
          <Box p="xl">
            <MessageBox variant="danger" message="Не удалось рассчитать статистику">
              Попробуйте ещё раз.
            </MessageBox>
            <Button mt="lg" variant="primary" onClick={run}>
              Повторить
            </Button>
          </Box>
        )}
      </Box>
    </Box>
  );
};

export default RecalculateStats;
