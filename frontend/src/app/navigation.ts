import { Bell, Boxes, ClipboardCheck, ClipboardList, LayoutDashboard, Package, ScanLine } from 'lucide-react';
import type { Role } from '../lib/api/types';

export const areas = [
  { path: '/', label: 'Overview', icon: LayoutDashboard, description: 'Your workspace, at a glance.' },
  { path: '/products', label: 'Products', icon: Package, description: 'A shared catalogue of your warehouse SKUs.', planned: 'Product listing and Supervisor product creation will be available here.' },
  { path: '/receivings', label: 'Receiving documents', icon: ClipboardList, description: 'Set expectations for every inbound delivery.', planned: 'Receiving documents, expected quantities and approved totals will be available here.' },
  { path: '/transactions', label: 'Receiving transactions', icon: ScanLine, description: 'Follow each receipt from recording to approval.', planned: 'Transaction history and role-permitted receiving actions will be available here.' },
  { path: '/inventory', label: 'Inventory', icon: Boxes, description: 'A clear record of approved inbound inventory.', planned: 'Approved balances and source-linked inventory movements will be available here.' },
  { path: '/notifications', label: 'Notifications', icon: Bell, description: 'Updates addressed to you, in one place.', planned: 'Your receiving updates and notification read actions will be available here.' },
];

export const reviewArea = { path: '/transactions/review', label: 'Supervisor review', icon: ClipboardCheck, description: 'Review receiving with confidence.', planned: 'Pending approvals and unresolved receiving reviews will be available here.' };
export const roleArea = (role: Role) => role === 'SUPERVISOR' ? reviewArea : areas[2]!;
