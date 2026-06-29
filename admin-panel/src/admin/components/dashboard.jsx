import { Box, Button, H2, Icon, Illustration, Text } from '@adminjs/design-system';
import { ApiClient } from 'adminjs';
import React, { useState } from 'react';

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
 * Дашборд админ-панели. Помимо приветствия, даёт кнопку выгрузки всей БД
 * в один Excel-файл, где каждая сущность — на отдельном листе.
 */
const Dashboard = () => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const exportAll = () => {
    setLoading(true);
    setError(false);
    api
      .getDashboard({ params: { format: 'xlsx' } })
      .then(({ data }) => {
        if (!data || !data.base64) {
          throw new Error('Пустой ответ сервера');
        }
        downloadBase64(data.base64, data.filename, data.mimeType);
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  };

  return (
    <Box variant="grey">
      <Box
        variant="white"
        flex
        flexDirection="column"
        alignItems="center"
        p="xxl"
      >
        <Illustration variant="DocumentCheck" />
        <H2 mt="lg">Выгрузка данных</H2>
        <Text mt="default" textAlign="center">
          Скачайте всю базу данных в один файл Excel. Каждая сущность —
          на отдельном листе.
        </Text>
        <Button
          mt="xl"
          variant="primary"
          size="lg"
          disabled={loading}
          onClick={exportAll}
        >
          <Icon icon="Download" />
          {loading ? 'Формируем файл…' : 'Скачать всю базу (Excel)'}
        </Button>
        {error && (
          <Text mt="lg" color="error">
            Не удалось сформировать файл. Попробуйте ещё раз.
          </Text>
        )}
      </Box>
    </Box>
  );
};

export default Dashboard;
