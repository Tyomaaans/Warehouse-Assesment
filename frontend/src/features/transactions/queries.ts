import type { QueryClient } from '@tanstack/react-query';
import { queryKeys } from '../../app/query-client';
import type { Transaction } from './api';

export const transactionKey = (actor: string, id: string) => [actor, 'transaction', id] as const;

// Only confirmed server responses seed transaction data. Receiving/inventory are always refetched.
export async function refreshReceiving(cache: QueryClient, actor: string, receiving: string, transaction?: Transaction) {
  await Promise.all([
    cache.invalidateQueries({ queryKey: queryKeys.receivings(actor) }),
    cache.invalidateQueries({ queryKey: queryKeys.transactions(actor, receiving) }),
    cache.invalidateQueries({ queryKey: queryKeys.inventory(actor) }),
    transaction ? cache.cancelQueries({ queryKey: transactionKey(actor, transaction.id) }) : Promise.resolve(),
  ]);
  if (transaction) cache.setQueryData(transactionKey(actor, transaction.id), transaction);
}
