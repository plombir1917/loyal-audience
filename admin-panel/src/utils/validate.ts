import { plainToInstance } from 'class-transformer';
import { validate } from 'class-validator';
import {
  ActionRequest,
  ValidationError as AdminValidationError,
} from 'adminjs';

export async function validateDto(dtoClass: any, payload: any) {
  const instance = plainToInstance(dtoClass, payload);

  const errors = await validate(instance, {
    whitelist: true,
  });

  if (errors.length > 0) {
    const formattedErrors = {};

    errors.forEach((err) => {
      if (err.constraints) {
        formattedErrors[err.property] = {
          message: Object.values(err.constraints)[0],
        };
      }

      // поддержка вложенных объектов (на будущее)
      if (err.children?.length) {
        err.children.forEach((child) => {
          if (child.constraints) {
            formattedErrors[`${err.property}.${child.property}`] = {
              message: Object.values(child.constraints)[0],
            };
          }
        });
      }
    });
    throw new AdminValidationError(formattedErrors);
  }

  return instance;
}

export async function validateRequest(
  request: ActionRequest,
  dtoClass: unknown,
) {
  const { payload = {}, method } = request;
  if (method !== 'post') return request;

  await validateDto(dtoClass, payload);

  return request;
}
