import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type PaginationProps = {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  totalItems?: number;
  itemsPerPage?: number;
};

function visiblePages(currentPage: number, totalPages: number) {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }

  const pages = new Set([1, totalPages, currentPage - 1, currentPage, currentPage + 1].filter((page) => page >= 1 && page <= totalPages));
  return Array.from(pages).sort((a, b) => a - b);
}

export function Pagination({ currentPage, totalPages, onPageChange, totalItems, itemsPerPage }: PaginationProps) {
  if (totalPages <= 1 && totalItems === undefined) {
    return null;
  }

  const start = totalItems !== undefined && itemsPerPage !== undefined ? Math.min((currentPage - 1) * itemsPerPage + 1, totalItems) : null;
  const end = totalItems !== undefined && itemsPerPage !== undefined ? Math.min(currentPage * itemsPerPage, totalItems) : null;
  const pages = visiblePages(currentPage, Math.max(totalPages, 1));

  return (
    <div className="flex flex-col gap-3 py-4 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
      {start !== null && end !== null ? <p>{`Showing ${start}-${end} of ${totalItems}`}</p> : <span />}
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="ghost" className="min-h-[44px]" disabled={currentPage <= 1} onClick={() => onPageChange(currentPage - 1)}>
          Previous
        </Button>
        {pages.map((page, index) => {
          const previous = pages[index - 1];
          return (
            <span key={page} className="flex items-center gap-2">
              {previous !== undefined && page - previous > 1 ? <span className="px-1">...</span> : null}
              <Button
                type="button"
                variant={page === currentPage ? 'default' : 'outline'}
                className={cn('min-h-[44px] min-w-[44px]', page === currentPage && 'bg-primary text-primary-foreground')}
                aria-current={page === currentPage ? 'page' : undefined}
                onClick={() => onPageChange(page)}
              >
                {page}
              </Button>
            </span>
          );
        })}
        <Button type="button" variant="ghost" className="min-h-[44px]" disabled={currentPage >= totalPages} onClick={() => onPageChange(currentPage + 1)}>
          Next
        </Button>
      </div>
    </div>
  );
}
