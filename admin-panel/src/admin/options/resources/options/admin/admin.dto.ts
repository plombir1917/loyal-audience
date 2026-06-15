import {
  IsEmail,
  IsNotEmpty,
  IsString,
  IsStrongPassword,
  MinLength,
} from 'class-validator';
import { validateRequest } from '../../../../../utils/validate.js';
import { ActionRequest } from 'adminjs';

export class AdminDTO {
  @IsEmail(
    {},
    {
      message: 'Некорректный e-mail.',
    },
  )
  email: string;

  @IsString({ message: 'Должно быть строкой.' })
  @IsNotEmpty()
  @MinLength(2, {
    message: 'Должно быть длиннее или равно 2 символам',
  })
  name: string;

  @IsStrongPassword({}, { message: 'Недостаточно сложный пароль.' })
  password: string;
}

export const validateUser = async (request: ActionRequest) => {
  return await validateRequest(request, AdminDTO);
};
