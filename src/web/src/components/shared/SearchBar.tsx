import { memo, useEffect, useRef, useState } from 'react';
import { RiCloseLine, RiSearchLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

export type SearchBarProps = {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  debounceMs?: number;
  className?: string;
};

export const SearchBar = memo(function SearchBar({ value, onChange, placeholder = 'Search...', debounceMs = 300, className }: SearchBarProps) {
  const [inputValue, setInputValue] = useState(value);
  const firstRender = useRef(true);

  useEffect(() => {
    setInputValue(value);
  }, [value]);

  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false;
      return undefined;
    }

    const timeout = window.setTimeout(() => {
      onChange(inputValue);
    }, debounceMs);

    return () => window.clearTimeout(timeout);
  }, [debounceMs, inputValue, onChange]);

  const clearSearch = () => {
    setInputValue('');
    onChange('');
  };

  return (
    <div className={cn('relative w-full min-w-[220px]', className)}>
      <RiSearchLine className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
      <Input
        value={inputValue}
        onChange={(event) => setInputValue(event.target.value)}
        placeholder={placeholder}
        className="min-h-[44px] rounded-lg pl-10 pr-11"
      />
      {inputValue ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Clear search"
          className="absolute right-1 top-1/2 min-h-9 -translate-y-1/2"
          onClick={clearSearch}
        >
          <RiCloseLine className="size-4" aria-hidden="true" />
        </Button>
      ) : null}
    </div>
  );
});
