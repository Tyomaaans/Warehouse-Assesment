export function pageOptions(params: URLSearchParams) {
  const page = Number(params.get('page') ?? 1);
  const limit = Number(params.get('limit') ?? 10);
  return { page: Number.isInteger(page) && page >= 1 && page <= 10000 ? page : 1, limit: [10, 25, 50, 100].includes(limit) ? limit : 10 };
}

export const displayDate = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });

export function focusInvalid(form: HTMLFormElement) {
  requestAnimationFrame(() => form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus());
}
