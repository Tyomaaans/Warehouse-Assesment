import type { Transaction } from './api';

export function TransactionStatus({ transaction }: { transaction: Transaction }) {
  const label = transaction.status === 'PENDING_APPROVAL' ? 'Pending approval' : transaction.status === 'APPROVED' ? 'Approved' : 'Rejected by system';
  return <span className="transaction-status"><span className={`status-tag ${transaction.status === 'APPROVED' ? 'completed' : transaction.status === 'REJECTED_BY_SYSTEM' ? 'rejected' : 'incomplete'}`}>{label}</span>{transaction.review_status && <span className="pill">{transaction.review_status === 'UNRESOLVED' ? 'Unresolved' : 'Resolved'}</span>}</span>;
}
