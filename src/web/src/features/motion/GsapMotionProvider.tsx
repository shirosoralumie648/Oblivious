import { type ReactNode, useRef } from 'react';
import { useGSAP } from '@gsap/react';
import gsap from 'gsap';

gsap.registerPlugin(useGSAP);

type GsapMotionProviderProps = {
  children: ReactNode;
};

type RafHandle = ReturnType<typeof window.requestAnimationFrame>;

function prefersReducedMotion() {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

function requestFrame(callback: FrameRequestCallback) {
  if (typeof window.requestAnimationFrame === 'function') {
    return window.requestAnimationFrame(callback);
  }

  return window.setTimeout(() => callback(Date.now()), 16) as unknown as RafHandle;
}

function cancelFrame(handle: RafHandle) {
  if (typeof window.cancelAnimationFrame === 'function') {
    window.cancelAnimationFrame(handle);
    return;
  }

  window.clearTimeout(handle as unknown as number);
}

function queryMotionItems(root: HTMLElement) {
  const selectors = [
    '[data-gsap-item]',
    '.workspace-canvas > section',
    '.workspace-canvas > section > *',
    '.console-canvas > section',
    '.console-canvas > section > *',
    '[data-gsap-scope="admin"] main > section',
    '[data-gsap-scope="admin"] main > section > *'
  ];

  return Array.from(new Set(root.querySelectorAll<HTMLElement>(selectors.join(',')))).filter((item) => item.dataset.gsapHidden !== 'true');
}

function hasElementMutations(mutations: MutationRecord[]) {
  return mutations.some((mutation) => Array.from(mutation.addedNodes).some((node) => node.nodeType === Node.ELEMENT_NODE));
}

export function GsapMotionProvider({ children }: GsapMotionProviderProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);

  useGSAP(
    (_context, contextSafe) => {
      const root = rootRef.current;

      if (!root || prefersReducedMotion()) {
        return;
      }

      const animatedItems = new WeakSet<HTMLElement>();
      const magneticCleanups = new WeakMap<HTMLElement, () => void>();
      let frame: RafHandle | null = null;

      const animateEntrance = () => {
        const items = queryMotionItems(root).filter((item) => !animatedItems.has(item));

        if (items.length === 0) {
          return;
        }

        items.forEach((item) => animatedItems.add(item));

        gsap.fromTo(
          items,
          { autoAlpha: 0, y: 18, scale: 0.985 },
          {
            autoAlpha: 1,
            clearProps: 'transform,opacity,visibility',
            duration: 0.58,
            ease: 'power3.out',
            overwrite: 'auto',
            scale: 1,
            stagger: { each: 0.045, from: 'start' },
            y: 0
          }
        );
      };

      const setupMagneticTargets = () => {
        root.querySelectorAll<HTMLElement>('[data-gsap-magnetic]').forEach((target) => {
          if (magneticCleanups.has(target)) {
            return;
          }

          const xTo = gsap.quickTo(target, 'x', { duration: 0.34, ease: 'power3.out' });
          const yTo = gsap.quickTo(target, 'y', { duration: 0.34, ease: 'power3.out' });
          const safe = contextSafe ?? (<T extends (...args: any[]) => unknown>(callback: T) => callback);

          const handlePointerMove = safe((event: PointerEvent) => {
            const bounds = target.getBoundingClientRect();
            const x = (event.clientX - bounds.left - bounds.width / 2) * 0.12;
            const y = (event.clientY - bounds.top - bounds.height / 2) * 0.16;

            xTo(x);
            yTo(y);
          });

          const handlePointerEnter = safe(() => {
            gsap.to(target, { duration: 0.22, ease: 'power2.out', overwrite: 'auto', scale: 1.018 });
          });

          const handlePointerLeave = safe(() => {
            xTo(0);
            yTo(0);
            gsap.to(target, { duration: 0.28, ease: 'power3.out', overwrite: 'auto', scale: 1 });
          });

          target.addEventListener('pointermove', handlePointerMove as EventListener);
          target.addEventListener('pointerenter', handlePointerEnter as EventListener);
          target.addEventListener('pointerleave', handlePointerLeave as EventListener);

          magneticCleanups.set(target, () => {
            target.removeEventListener('pointermove', handlePointerMove as EventListener);
            target.removeEventListener('pointerenter', handlePointerEnter as EventListener);
            target.removeEventListener('pointerleave', handlePointerLeave as EventListener);
            gsap.killTweensOf(target);
          });
        });
      };

      const refresh = () => {
        animateEntrance();
        setupMagneticTargets();
      };
      const safeRefresh = contextSafe ? contextSafe(refresh) : refresh;

      const scheduleRefresh = () => {
        if (frame !== null) {
          cancelFrame(frame);
        }

        frame = requestFrame(() => {
          frame = null;
          safeRefresh();
        });
      };

      const observer = new MutationObserver((mutations) => {
        if (hasElementMutations(mutations)) {
          scheduleRefresh();
        }
      });

      scheduleRefresh();
      observer.observe(root, { childList: true, subtree: true });

      return () => {
        if (frame !== null) {
          cancelFrame(frame);
        }

        observer.disconnect();
        root.querySelectorAll<HTMLElement>('[data-gsap-magnetic]').forEach((target) => {
          magneticCleanups.get(target)?.();
        });
      };
    },
    { scope: rootRef }
  );

  return (
    <div className="motion-root min-h-screen" ref={rootRef}>
      {children}
    </div>
  );
}
