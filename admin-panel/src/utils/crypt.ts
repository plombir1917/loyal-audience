import * as bcrypt from 'bcrypt';
import * as crypto from 'crypto';

export async function encodePassword(raw: string) {
  const salt = await bcrypt.genSalt();
  return bcrypt.hash(raw, salt);
}

export async function comparePassword(raw: string, encoded: string) {
  return bcrypt.compare(raw, encoded);
}

/**
 * Хэширует API-ключ алгоритмом SHA-256.
 *
 * @description
 * SHA-256 формирует 256-битный криптографический digest,
 * который кодируется в виде 64 hex-символов.
 *
 * Используется для безопасного хранения API-ключей в БД:
 * вместо оригинального ключа сохраняется только его hash.
 *
 * @param raw - API-ключ в открытом виде
 * @returns SHA-256 hash ключа в hex-формате
 */
export function hashAPIKey(raw: string): string {
  return crypto.createHash('sha256').update(raw).digest('hex');
}

/**
 * Проверяет корректность API-ключа через сравнение SHA-256 hash.
 *
 * @description
 * Открытый API-ключ повторно хешируется алгоритмом SHA-256,
 * после чего результат сравнивается с сохранённым hash.
 *
 * Для сравнения используется timingSafeEqual(),
 * предотвращающий timing attack — атаку,
 * основанную на измерении времени сравнения строк.
 *
 * @param raw - API-ключ в открытом виде
 * @param hashed - сохранённый SHA-256 hash API-ключа
 * @returns true, если ключ валиден, иначе false
 */
export function verifyAPIKey(raw: string, hashed: string): boolean {
  const rawHash = hashAPIKey(raw);

  return crypto.timingSafeEqual(Buffer.from(rawHash), Buffer.from(hashed));
}

/**
 * Генерирует криптографически стойкий API-ключ.
 *
 * @description
 * API-ключ состоит из:
 * - публичного prefix-идентификатора
 * - секретной случайной части
 *
 * Формат ключа:
 * vas_live_<prefix>.<secret>
 *
 * Где:
 * - prefix содержит 4 случайных байта
 *   и кодируется в 8 hex-символов
 * - secret содержит 32 случайных байта
 *   и кодируется в 64 hex-символа
 *
 * Генерация выполняется через crypto.randomBytes(),
 * обеспечивающий криптографически безопасную случайность.
 *
 * Итоговая энтропия secret-части составляет 256 бит,
 * что делает brute-force подбор практически невозможным.
 *
 * @returns API-ключ в открытом виде
 */
export function generateAPIKey(): string {
  const prefix = crypto.randomBytes(4).toString('hex');

  const secret = crypto.randomBytes(32).toString('hex');

  return `vas_live_${prefix}.${secret}`;
}
