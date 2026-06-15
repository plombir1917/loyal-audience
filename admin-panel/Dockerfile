# стейдж билда
FROM node:20-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .

RUN npm run generate
RUN npm run build

# стейдж прода
FROM node:20-alpine

WORKDIR /app

COPY package*.json ./

RUN npm ci --omit=dev

# скомпилированные файлы и необходимые ресурсы
COPY --from=builder /app/dist ./dist

# статика для фронтенда и админки
COPY --from=builder /app/src/admin/components/ ./src/admin/components/

# копируем папку prisma, т.к. файл со схемой не компилируется и генерируем клиент
COPY --from=builder /app/prisma/ ./prisma
RUN npx prisma generate

EXPOSE 3000

CMD ["npm", "run", "start:prod"]