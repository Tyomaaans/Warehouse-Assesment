import type { ReactNode } from 'react';

// Semantics and horizontal overflow are shared; feature pages supply actual rows later.
export function Table({ caption, headings, children }: { caption: string; headings: string[]; children: ReactNode }) {
  return <div className="table-scroll" tabIndex={0} role="region" aria-label={caption}><table>
    <caption className="sr-only">{caption}</caption><thead><tr>{headings.map(heading => <th scope="col" key={heading}>{heading}</th>)}</tr></thead><tbody>{children}</tbody>
  </table></div>;
}
