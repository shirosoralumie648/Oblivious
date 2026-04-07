import { Link } from 'react-router-dom';

type ConsoleOverviewCardProps = {
  title: string;
  value: string;
  note: string;
  to: string;
};

export function ConsoleOverviewCard({ title, value, note, to }: ConsoleOverviewCardProps) {
  return (
    <Link aria-label={title} to={to}>
      <h2>{title}</h2>
      <p>{value}</p>
      <p>{note}</p>
    </Link>
  );
}
