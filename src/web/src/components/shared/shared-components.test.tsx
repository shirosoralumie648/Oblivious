import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { DataTable, type DataTableColumn } from './DataTable';
import { Pagination } from './Pagination';
import { RatingStars } from './RatingStars';
import { SearchBar } from './SearchBar';
import { StatusBadge } from './StatusBadge';

type Row = {
  id: string;
  name: string;
  status: string;
};

const columns: DataTableColumn<Row>[] = [
  { key: 'name', header: 'Name', sortable: true },
  { key: 'status', header: 'Status' },
];

describe('shared components', () => {
  it('renders DataTable loading, error, empty, and populated states', () => {
    const { rerender } = render(
      <DataTable<Row> columns={columns} data={[]} loading error={null} emptyMessage="No rows" />
    );
    expect(screen.getByLabelText('Loading table data')).toBeInTheDocument();

    rerender(<DataTable<Row> columns={columns} data={[]} loading={false} error="Nope" emptyMessage="No rows" />);
    expect(screen.getByText('Something went wrong while loading this data. Please try again or contact support if the issue persists.')).toBeInTheDocument();
    expect(screen.getByText('Nope')).toBeInTheDocument();

    rerender(<DataTable<Row> columns={columns} data={[]} loading={false} error={null} emptyMessage="No rows" />);
    expect(screen.getByText('No rows')).toBeInTheDocument();

    rerender(
      <DataTable<Row>
        columns={columns}
        data={[{ id: 'row_1', name: 'Alpha', status: 'online' }]}
        loading={false}
        error={null}
        emptyMessage="No rows"
      />
    );
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'Alpha' })).toBeInTheDocument();
  });

  it('supports DataTable sorting and selection callbacks', () => {
    const onSort = vi.fn();
    const onSelectChange = vi.fn();

    render(
      <DataTable<Row>
        columns={columns}
        data={[{ id: 'row_1', name: 'Alpha', status: 'online' }]}
        loading={false}
        error={null}
        emptyMessage="No rows"
        onSort={onSort}
        selectable
        selectedIds={new Set()}
        onSelectChange={onSelectChange}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Sort by Name' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select row Alpha' }));

    expect(onSort).toHaveBeenCalledWith('name');
    expect(onSelectChange).toHaveBeenCalledWith('row_1', true);
  });

  it('debounces SearchBar updates and clears input', async () => {
    vi.useFakeTimers();
    const onChange = vi.fn();

    try {
      render(<SearchBar value="" onChange={onChange} debounceMs={200} />);

      fireEvent.change(screen.getByPlaceholderText('Search...'), { target: { value: 'agent' } });
      vi.advanceTimersByTime(199);
      expect(onChange).not.toHaveBeenCalled();
      vi.advanceTimersByTime(1);
      expect(onChange).toHaveBeenCalledWith('agent');

      fireEvent.click(screen.getByRole('button', { name: 'Clear search' }));
      expect(onChange).toHaveBeenCalledWith('');
    } finally {
      vi.useRealTimers();
    }
  });

  it('renders StatusBadge and interactive RatingStars accessibly', () => {
    const onChange = vi.fn();

    render(
      <>
        <StatusBadge status="online" />
        <RatingStars value={3.5} onChange={onChange} showValue count={12} />
      </>
    );

    expect(screen.getByLabelText('Online')).toBeInTheDocument();
    expect(screen.getByText('3.5 (12 reviews)')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('radio', { name: 'Rate 4 out of 5 stars' }));
    expect(onChange).toHaveBeenCalledWith(4);
  });

  it('renders Pagination accessibly', () => {
    const onPageChange = vi.fn();
    render(<Pagination currentPage={2} totalPages={5} totalItems={50} itemsPerPage={10} onPageChange={onPageChange} />);

    expect(screen.getByRole('navigation', { name: 'Pagination' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to previous page' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to next page' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to page 1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to page 2' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    expect(onPageChange).toHaveBeenCalledWith(3);
  });
});
