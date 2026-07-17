// ============================================================
// nav.ax FormField — shared input/textarea/select with consistent styling
// ============================================================
import { forwardRef } from 'react';
import { cn } from '@/lib/utils';

interface FormFieldProps {
  label: string;
  children: React.ReactNode;
  className?: string;
}

export function FormField({ label, children, className }: FormFieldProps) {
  return (
    <div className={className}>
      <label className="block text-xs font-medium text-foreground-500 mb-1.5 tracking-wide">{label}</label>
      {children}
    </div>
  );
}

// ---- Shared input base styles ----
const inputBase = 'form-glow w-full h-9 px-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200/60 transition-all duration-200';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  className?: string;
}

export const FormInput = forwardRef<HTMLInputElement, InputProps>(function FormInput(
  { className, ...props },
  ref,
) {
  return <input ref={ref} {...props} className={cn(inputBase, className)} />;
});

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  className?: string;
}

export function FormSelect({ className, children, ...props }: SelectProps) {
  return (
    <select {...props} className={cn(inputBase, 'appearance-none cursor-pointer', className)}>
      {children}
    </select>
  );
}

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  className?: string;
}

export function FormTextarea({ className, ...props }: TextareaProps) {
  return (
    <textarea
      {...props}
      maxLength={props.maxLength || 500}
      className={cn(inputBase, 'h-auto py-2 resize-none', className)}
    />
  );
}

// ---- Search input (slightly different styling — thinner, for inline use) ----
export function SearchInput({ className, ...props }: InputProps) {
  return (
    <div className="relative">
      <i className="ri-search-line absolute left-3 top-1/2 -translate-y-1/2 text-sm text-foreground-300 pointer-events-none" />
      <input
        {...props}
        type="text"
        className={cn(
          'w-full h-9 pl-9 pr-3 rounded-lg bg-background-50 border border-background-200/70 text-sm text-foreground-900 placeholder:text-foreground-300 focus:outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-200/60 form-glow transition-all duration-200',
          className,
        )}
      />
    </div>
  );
}
