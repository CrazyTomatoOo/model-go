export type ConsoleUser = {
  id: number;
  username: string;
  /** Optional display name; the shell avatar prefers it over username (#36). */
  display_name?: string | null;
  role: 'user' | 'sysadmin' | 'secadmin' | 'auditadmin';
  status: 'pending' | 'active' | 'disabled';
  must_change_password: boolean;
};
