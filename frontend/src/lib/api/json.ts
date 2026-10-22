// Keep the contract's int64 quantities exact, including values above Number.MAX_SAFE_INTEGER.
// JSON source access/rawJSON are supported by the tested Node and Chrome toolchains.
const quantityKeys = new Set(['expected_quantity', 'approved_quantity', 'remaining_quantity', 'actual_quantity']);
type SourceJSON = typeof JSON & { rawJSON?: (value: string) => unknown };

export function encodeJSON(value: unknown): string {
  return JSON.stringify(value, (_key, item: unknown) => {
    if (typeof item !== 'bigint') return item;
    const rawJSON = (JSON as SourceJSON).rawJSON;
    if (!rawJSON) {
      if (Number.isSafeInteger(Number(item))) return Number(item);
      throw new Error('This browser needs an update to send exact quantities.');
    }
    return rawJSON(item.toString());
  });
}

export function decodeJSON(text: string): unknown {
  return JSON.parse(text, (key: string, value: unknown, context?: { source?: string }) => {
    if (!quantityKeys.has(key) || typeof value !== 'number') return value;
    if (context?.source && /^\d+$/.test(context.source)) return BigInt(context.source);
    if (Number.isSafeInteger(value)) return BigInt(value);
    throw new Error('This browser cannot read this quantity exactly.');
  });
}
