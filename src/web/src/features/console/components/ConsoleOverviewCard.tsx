import { Link } from 'react-router-dom';

type ConsoleOverviewCardProps = {
  title: string;
  value: string;
  note: string;
  to: string;
};

export function ConsoleOverviewCard({ title, value, note, to }: ConsoleOverviewCardProps) {
  return (
    <Link
      aria-label={title}
      className="block min-h-[144px] rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5 shadow-sm transition hover:border-[#1a614f]/40 hover:bg-[#e9f2ee]"
      data-gsap-item
      data-gsap-magnetic
      to={to}
    >
      <h2 className="text-sm font-semibold text-[#1a614f]">{title}</h2>
      <p className="mt-3 text-2xl font-semibold text-[#181611]">{value}</p>
      <p className="mt-2 text-sm leading-6 text-[#625b4f]">{note}</p>
    </Link>
  );
}
