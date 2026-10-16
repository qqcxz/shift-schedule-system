export function getLoginRedirect(value: unknown): string {
  if (
    typeof value !== 'string' ||
    !value.startsWith('/') ||
    value.startsWith('//') ||
    /[\\\u0000-\u001f\u007f]/.test(value) ||
    /^\/(?:login|change-password)(?:[/?#]|$)/i.test(value)
  ) {
    return '/schedule';
  }
  return value;
}
