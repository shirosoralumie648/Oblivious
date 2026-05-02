import { useState } from 'react';
import { RiCloseLine } from '@remixicon/react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

export type FilterPanelProps = {
  categories: { slug: string; name: string; count: number }[];
  selectedCategories: string[];
  onCategoryChange: (slugs: string[]) => void;
  selectedTags: string[];
  availableTags: string[];
  onTagsChange: (tags: string[]) => void;
  minRating: number;
  onRatingChange: (rating: number) => void;
  priceFilter: 'all' | 'free' | 'paid';
  onPriceFilterChange: (filter: 'all' | 'free' | 'paid') => void;
  className?: string;
};

export function FilterPanel({
  categories,
  selectedCategories,
  onCategoryChange,
  selectedTags,
  availableTags,
  onTagsChange,
  minRating,
  onRatingChange,
  priceFilter,
  onPriceFilterChange,
  className,
}: FilterPanelProps) {
  const [tagInput, setTagInput] = useState('');

  const toggleCategory = (slug: string, checked: boolean) => {
    onCategoryChange(checked ? [...selectedCategories, slug] : selectedCategories.filter((item) => item !== slug));
  };

  const addTag = (tag: string) => {
    const normalized = tag.trim();
    if (normalized && !selectedTags.includes(normalized)) {
      onTagsChange([...selectedTags, normalized]);
    }
    setTagInput('');
  };

  return (
    <Card className={cn('rounded-lg', className)}>
      <CardHeader>
        <CardTitle>Filters</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <section className="space-y-3">
          <h3 className="text-sm font-medium text-foreground">Categories</h3>
          <div className="space-y-2">
            {categories.map((category) => (
              <label key={category.slug} className="flex min-h-[44px] items-center justify-between gap-3 rounded-lg px-2 hover:bg-muted/40">
                <span className="flex items-center gap-3">
                  <Checkbox
                    checked={selectedCategories.includes(category.slug)}
                    onCheckedChange={(checked) => toggleCategory(category.slug, checked === true)}
                  />
                  <span className="text-sm">{category.name}</span>
                </span>
                <Badge variant="outline">{category.count}</Badge>
              </label>
            ))}
          </div>
        </section>

        <section className="space-y-3">
          <h3 className="text-sm font-medium text-foreground">Tags</h3>
          <div className="flex flex-wrap gap-2">
            {selectedTags.map((tag) => (
              <Badge key={tag} variant="secondary" className="min-h-7">
                {tag}
                <button type="button" aria-label={`Remove ${tag}`} onClick={() => onTagsChange(selectedTags.filter((item) => item !== tag))}>
                  <RiCloseLine className="size-3" aria-hidden="true" />
                </button>
              </Badge>
            ))}
          </div>
          <Input
            list="available-marketplace-tags"
            value={tagInput}
            onChange={(event) => setTagInput(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                addTag(tagInput);
              }
            }}
            placeholder="Add tag..."
            className="min-h-[44px] rounded-lg"
          />
          <datalist id="available-marketplace-tags">
            {availableTags.map((tag) => (
              <option key={tag} value={tag} />
            ))}
          </datalist>
        </section>

        <section className="space-y-3">
          <h3 className="text-sm font-medium text-foreground">Rating</h3>
          <div className="grid grid-cols-4 gap-2">
            {[0, 3, 4, 4.5].map((rating) => (
              <Button
                key={rating}
                type="button"
                variant={minRating === rating ? 'default' : 'outline'}
                className="min-h-[44px]"
                onClick={() => onRatingChange(rating)}
              >
                {rating === 0 ? 'Any' : `${rating}+`}
              </Button>
            ))}
          </div>
        </section>

        <section className="space-y-3">
          <h3 className="text-sm font-medium text-foreground">Price</h3>
          <div className="grid grid-cols-3 gap-2">
            {(['all', 'free', 'paid'] as const).map((filter) => (
              <Button
                key={filter}
                type="button"
                variant={priceFilter === filter ? 'default' : 'outline'}
                className="min-h-[44px] capitalize"
                onClick={() => onPriceFilterChange(filter)}
              >
                {filter}
              </Button>
            ))}
          </div>
        </section>
      </CardContent>
    </Card>
  );
}
