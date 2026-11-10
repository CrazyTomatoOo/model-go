import { useTranslation } from 'react-i18next';

const POLICY_ITEMS = ['lengthClasses', 'exclusions', 'history'] as const;

/**
 * The password policy as a structured three-item list (#35), shared by the
 * standalone /change-password page and the shell change-password modal.
 */
export default function PasswordPolicyList() {
  const { t } = useTranslation();
  return (
    <ul className="password-policy-list">
      {POLICY_ITEMS.map((item) => (
        <li key={item}>{t(`changePassword.policyList.${item}`)}</li>
      ))}
    </ul>
  );
}
