import { Injectable } from '@nestjs/common';
import {
  ActionRequest,
  ActionResponse,
  BaseRecord,
  ResourceOptions,
} from 'adminjs';
import { AuthService } from '../../../../auth/auth.service.js';
import { encodePassword } from '../../../../../utils/crypt.js';

@Injectable()
export class AdminOptions {
  constructor(private readonly authService: AuthService) {}

  get(): ResourceOptions {
    return {
      navigation: {
        icon: 'User',
      },
      properties: {
        created_at: {
          isVisible: false,
        },
      },
      actions: {
        new: {
          before: [hashPassword],
        },
        show: {
          after: [hidePassword],
        },
        list: {
          after: [hidePassword],
        },
      },
    };
  }
}

async function hashPassword(context: ActionRequest) {
  if (context?.payload?.password) {
    context.payload = {
      ...context.payload,
      password: await encodePassword(context.payload.password),
    };
  }
  return context;
}

function hidePassword(response: ActionResponse) {
  if (response?.records) {
    response.records?.forEach((record: BaseRecord) => {
      if (record?.params?.password) delete record.params.password;
    });
  } else if (response?.record?.params?.password) {
    delete response.record.params.password;
  }
  return response;
}
