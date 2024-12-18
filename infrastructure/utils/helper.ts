enum ACCOUNTS {
  PRODUCTION = 'common', // common
  DEVELOPMENT = 'dev', // dev
}

export function getVwsServiceRoleArn(accountId: string): string {
  const roleName = 'CloudFormationRegistration'
  let roleId;
  switch (getAccountNameById(accountId)) {
    case ACCOUNTS.PRODUCTION:
      roleId = 'bb2b2d';
      break;
    case ACCOUNTS.DEVELOPMENT:
      roleId = '96a56e';
      break;
    default:
      throw new Error('VWS service role arn could not be mapped');
  }

  return `arn:aws:iam::${accountId}:role/vws/initializer/vws-init-${roleId}-${roleName}`
}

export function getVwsServiceQueueArn(accountId: string): string {
  let queueId;
  switch (getAccountNameById(accountId)) {
    case ACCOUNTS.PRODUCTION:
      queueId = '3c678860-f9b2-4872-9be4-8ef45275635f';
      break;
    case ACCOUNTS.DEVELOPMENT:
      queueId = '872ebdaa-5b77-4f13-9e29-af25c9db4123';
      break;
    default:
      throw new Error('VWS service queue arn could not be mapped');
  }

  return `arn:aws:sqs:eu-west-1:685456541949:service-notification-${accountId}-${queueId}.fifo`
}

export function getAccountNameById(accountId: string): ACCOUNTS {
  switch (accountId) {
    case '773250545321':
      return ACCOUNTS.PRODUCTION;
    case '407876406117':
      return ACCOUNTS.DEVELOPMENT;
    default:
      throw new Error(`Account with id ${accountId} not found`);
  }
}

export function createVpcName(accountId: string): string {
  const accountName = getAccountNameById(accountId);

  if(accountName === ACCOUNTS.PRODUCTION) {
    return 'orma-vpc'
  }
  return `${accountName}-order`;
}