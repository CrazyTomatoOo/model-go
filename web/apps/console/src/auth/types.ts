export type ConsoleUser = {
  id: number;
  username: string;
  role: 'user' | 'sysadmin' | 'secadmin' | 'auditadmin';
  status: 'pending' | 'active' | 'disabled';
  must_change_password: boolean;
};
