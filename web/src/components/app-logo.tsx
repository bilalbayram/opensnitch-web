import type { ComponentProps } from 'react';
import { cn } from '@/lib/utils';

type AppLogoProps = Omit<ComponentProps<'img'>, 'src'>;

export function AppLogo({ alt = '', className, ...props }: AppLogoProps) {
  return (
    <img
      src="/icon.svg"
      alt={alt}
      className={cn('shrink-0 select-none', className)}
      draggable={false}
      {...props}
    />
  );
}
