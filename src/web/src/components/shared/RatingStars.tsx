import { memo, useState } from 'react';
import { RiStarFill, RiStarHalfFill, RiStarLine } from '@remixicon/react';

import { cn } from '@/lib/utils';

export type RatingStarsProps = {
  value: number;
  onChange?: (value: number) => void;
  readonly?: boolean;
  size?: 'sm' | 'md' | 'lg';
  showValue?: boolean;
  count?: number;
  className?: string;
};

const sizeClass = {
  sm: 'size-4',
  md: 'size-5',
  lg: 'size-6',
};

function starIcon(star: number, value: number, className: string) {
  const delta = value - (star - 1);
  if (delta >= 0.75) {
    return <RiStarFill className={className} aria-hidden="true" />;
  }
  if (delta >= 0.25) {
    return <RiStarHalfFill className={className} aria-hidden="true" />;
  }
  return <RiStarLine className={className} aria-hidden="true" />;
}

function clampRating(value: number) {
  return Math.max(0, Math.min(5, Math.round(value * 2) / 2));
}

export const RatingStars = memo(function RatingStars({ value, onChange, readonly = onChange === undefined, size = 'md', showValue = false, count, className }: RatingStarsProps) {
  const [preview, setPreview] = useState<number | null>(null);
  const displayValue = clampRating(preview ?? value);
  const interactive = !readonly && onChange !== undefined;
  const formattedValue = value.toFixed(1);

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (!interactive) {
      return;
    }

    if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
      event.preventDefault();
      onChange(clampRating(value + 0.5));
    }
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
      event.preventDefault();
      onChange(clampRating(value - 0.5));
    }
  };

  return (
    <div className={cn('inline-flex items-center gap-2', className)}>
      <div
        role={interactive ? 'radiogroup' : 'img'}
        aria-label={interactive ? 'Rating' : `${formattedValue} out of 5 stars`}
        className="inline-flex items-center gap-1"
        onMouseLeave={() => setPreview(null)}
        onKeyDown={handleKeyDown}
      >
        {[1, 2, 3, 4, 5].map((star) => {
          const checked = Math.round(value) === star;
          const iconClass = cn(sizeClass[size], displayValue >= star - 0.75 ? 'text-[oklch(0.795_0.184_86.047)]' : 'text-muted-foreground');

          if (!interactive) {
            return <span key={star}>{starIcon(star, displayValue, iconClass)}</span>;
          }

          return (
            <button
              key={star}
              type="button"
              role="radio"
              aria-checked={checked}
              aria-label={`Rate ${star} out of 5 stars`}
              className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-lg outline-none transition-colors hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50"
              onMouseEnter={() => setPreview(star)}
              onFocus={() => setPreview(star)}
              onBlur={() => setPreview(null)}
              onClick={() => onChange(star)}
            >
              {starIcon(star, displayValue, iconClass)}
            </button>
          );
        })}
      </div>
      {showValue ? (
        <span className="text-sm text-muted-foreground">
          {formattedValue}
          {count !== undefined ? ` (${count.toLocaleString()} reviews)` : null}
        </span>
      ) : null}
    </div>
   );
});
