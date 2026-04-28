import { useTranslation } from 'react-i18next';
import { cn } from '@/utils/cn';

interface Props {
  current: 1 | 2 | 3;
}

export const StepProgress = ({ current }: Props) => {
  const { t } = useTranslation();
  const steps = [
    { n: 1, label: t('step.guest') },
    { n: 2, label: t('step.firm') },
    { n: 3, label: t('step.review') },
  ];
  return (
    <ol className="flex items-center gap-3">
      {steps.map((s, i) => (
        <li key={s.n} className="flex items-center gap-3">
          <span
            className={cn(
              'flex h-9 w-9 items-center justify-center rounded-full border text-sm font-semibold',
              s.n === current
                ? 'border-brand bg-brand text-white'
                : s.n < current
                  ? 'border-brand bg-brand-soft text-brand'
                  : 'border-border-subtle bg-surface text-ink-muted',
            )}
          >
            {s.n}
          </span>
          <span
            className={cn(
              'hidden text-sm font-medium tablet:inline',
              s.n === current ? 'text-ink' : 'text-ink-muted',
            )}
          >
            {s.label}
          </span>
          {i < steps.length - 1 && <span className="h-px w-8 bg-border-subtle" aria-hidden />}
        </li>
      ))}
    </ol>
  );
};
