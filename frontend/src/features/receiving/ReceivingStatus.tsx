import type { ReceivingStatus as Status } from './api';

export function ReceivingStatus({ status }: { status: Status }) {
  return <span className={`status-tag ${status === 'COMPLETED' ? 'completed' : 'incomplete'}`}>{status === 'COMPLETED' ? 'Completed' : 'Incomplete'}</span>;
}
