import { ReactNode, useId } from 'react';
import { cn } from '@/utils/cn';

interface Props {
  label: string;
  required?: boolean;
  error?: string;
  hint?: string;
  className?: string;
  children: (id: string) => ReactNode;
}

// Lightweight wrapper used by every form input. Big label, big spacing, error
// message in the brand red. The `children` render-prop receives the id so we
// can wire htmlFor properly without prop drilling.
export const Field = ({ label, required, error, hint, className, children }: Props) => {
  const id = useId();
  return (
    <div className={cn('flex flex-col gap-2', className)}>
      <label htmlFor={id} className="flex items-baseline gap-2 text-base font-medium text-ink">
        {label}
        {required && <span className="text-accent-strong">*</span>}
      </label>
      {children(id)}
      {hint && !error && <p className="text-sm text-ink-muted">{hint}</p>}
      {error && <p className="text-sm font-medium text-red-600">{error}</p>}
    </div>
  );
};
