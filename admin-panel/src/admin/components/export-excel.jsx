import { Box, Button, Loader, MessageBox, Text } from '@adminjs/design-system';
import { ApiClient } from 'adminjs';
import React, { useCallback, useEffect, useState } from 'react';

const api = new ApiClient();

// base64 -> Blob -> скачивание файла в браузере.
const downloadBase64 = (base64, filename, mimeType) => {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  const blob = new Blob([bytes], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

/**
 * Страница resource-action «Excel»: при открытии запрашивает у сервера
 * выгрузку таблицы и сразу скачивает .xlsx-файл.
 */
const ExportExcel = ({ resource, action }) => {
  const [status, setStatus] = useState('loading'); // loading | done | error

  const run = useCallback(() => {
    setStatus('loading');
    api
      .resourceAction({ resourceId: resource.id, actionName: action.name })
      .then(({ data }) => {
        if (!data || !data.base64) {
          throw new Error('Пустой ответ сервера');
        }
        downloadBase64(data.base64, data.filename, data.mimeType);
        setStatus('done');
      })
      .catch(() => setStatus('error'));
  }, [resource.id, action.name]);

  useEffect(() => {
    run();
  }, [run]);

  return (
    <Box variant="grey">
      <Box variant="white">
        {status === 'loading' && (
          <Box flex flexDirection="column" alignItems="center" p="xxl">
            <Loader />
            <Text mt="lg">Формируем файл Excel…</Text>
          </Box>
        )}

        {status === 'done' && (
          <Box p="xl">
            <MessageBox
              variant="success"
              message="Файл готов"
              data-testid="export-success"
            >
              Скачивание началось автоматически. Если этого не произошло —
              нажмите кнопку ниже.
            </MessageBox>
            <Button mt="lg" variant="primary" onClick={run}>
              Скачать снова
            </Button>
          </Box>
        )}

        {status === 'error' && (
          <Box p="xl">
            <MessageBox variant="danger" message="Не удалось сформировать файл">
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

export default ExportExcel;
