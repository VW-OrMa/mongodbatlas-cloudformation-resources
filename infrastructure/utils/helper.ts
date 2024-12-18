enum ACCOUNTS {
  COMMON = 'common', // common
  DEV = 'dev', // dev
}

export function getAccountNameById(accountId: string): ACCOUNTS {
  switch (accountId) {
    case '773250545321':
      return ACCOUNTS.COMMON;
    case '407876406117':
      return ACCOUNTS.DEV;
    default:
      throw new Error(`Account with id ${accountId} not found`);
  }
}

export function createVpcName(accountId: string): string {
  const accountName = getAccountNameById(accountId);

  if(accountName === ACCOUNTS.COMMON) {
    return 'orma-vpc'
  }
  return `${accountName}-order`;
}