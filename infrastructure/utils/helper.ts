enum ACCOUNTS {
  PRODUCTION = 'common', // common
  DEVELOPMENT = 'dev', // dev
}

export function getVwsServiceQueueArn(accountId: string): string {
  let queueId;
  switch (getAccountNameById(accountId)) {
    case ACCOUNTS.PRODUCTION:
      queueId = '4b5e16d0-9b2a-47eb-a050-a92786cc4f8a';
      break;
    case ACCOUNTS.DEVELOPMENT:
      queueId = 'd7969cfe-9bc3-45c9-9820-454daef4f43b';
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