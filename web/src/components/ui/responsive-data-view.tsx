import type { ReactNode } from 'react';

interface ResponsiveDataViewProps<T> {
  data: T[];
  /** Desktop table header */
  tableHead: ReactNode;
  /** Desktop table row renderer */
  renderRow: (item: T, index: number) => ReactNode;
  /** Mobile card renderer */
  renderCard: (item: T, index: number) => ReactNode;
  /** Table column count (for empty state colSpan) */
  columns: number;
  /** Message when data is empty */
  emptyMessage?: string;
  /** Extra class on the outer wrapper */
  className?: string;
}

export function ResponsiveDataView<T>({
  data,
  tableHead,
  renderRow,
  renderCard,
  columns,
  emptyMessage = 'No data',
  className = '',
}: ResponsiveDataViewProps<T>) {
  return (
    <>
      {/* Desktop: table */}
      <div className={`hidden lg:block bg-card border border-border rounded-xl overflow-hidden ${className}`}>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              {tableHead}
            </thead>
            <tbody>
              {data.map((item, i) => renderRow(item, i))}
              {data.length === 0 && (
                <tr>
                  <td colSpan={columns} className="px-4 py-8 text-center text-muted-foreground">
                    {emptyMessage}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Mobile: cards */}
      <div className={`lg:hidden space-y-2 ${className}`}>
        {data.map((item, i) => renderCard(item, i))}
        {data.length === 0 && (
          <div className="bg-card border border-border rounded-xl px-4 py-8 text-center text-muted-foreground">
            {emptyMessage}
          </div>
        )}
      </div>
    </>
  );
}
