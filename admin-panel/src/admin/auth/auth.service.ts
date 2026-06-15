import { Injectable, UnauthorizedException } from '@nestjs/common';
import { comparePassword } from '../../utils/crypt.js';
import { ActionContext } from 'adminjs';
import { PrismaService } from '../../../prisma/prisma.service.js';

@Injectable()
export class AuthService {
  constructor(private readonly prismaService: PrismaService) {}

  async login(email: string, rawPassword: string) {
    try {
      const admin = await this.prismaService.admin.findUnique({
        where: { email },
      });
      // Пользователи, зарегистрированные через VK ID, не имеют пароля и входят
      // только через VK ID — форму email/пароль для них не пропускаем.
      if (
        admin &&
        admin.password &&
        (await comparePassword(rawPassword, admin.password))
      ) {
        return { id: admin.id, email: admin.email };
      }
      return null;
    } catch (error) {
      throw new UnauthorizedException(error);
    }
  }
}

export const currentadminIsAdmin = ({ currentAdmin }: ActionContext) => {
  return !!currentAdmin && currentAdmin?.role === 'admin';
};
