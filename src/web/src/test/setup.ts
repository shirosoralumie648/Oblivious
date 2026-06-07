import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';

type ConsoleMethod = 'warn' | 'error';

interface UnexpectedConsoleCall {
  method: ConsoleMethod;
  args: unknown[];
}

class ResizeObserverMock implements ResizeObserver {
  private readonly callback: ResizeObserverCallback;

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
  }

  disconnect() {}
  observe(target: Element, _options?: ResizeObserverOptions) {
    const rect = target.getBoundingClientRect();
    const width = rect.width || 820;
    const height = rect.height || 520;
    const entry = {
      borderBoxSize: [{ blockSize: height, inlineSize: width }],
      contentBoxSize: [{ blockSize: height, inlineSize: width }],
      contentRect: {
        bottom: height,
        height,
        left: 0,
        right: width,
        top: 0,
        width,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      },
      devicePixelContentBoxSize: [{ blockSize: height, inlineSize: width }],
      target,
    } as ResizeObserverEntry;

    this.callback([entry], this);
  }
  unobserve(_target: Element) {}
}

class DOMMatrixReadOnlyMock {
  readonly m22: number;

  constructor(transform?: string) {
    const scaleMatch = transform?.match(/scale\(([-\d.]+)\)/);
    const matrixMatch = transform?.match(/matrix\(([^)]+)\)/);
    const matrixValues = matrixMatch?.[1]
      .split(',')
      .map((value) => Number(value.trim()))
      .filter((value) => Number.isFinite(value));
    this.m22 = scaleMatch ? Number(scaleMatch[1]) : (matrixValues?.[3] ?? 1);
  }
}

if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = ResizeObserverMock;
}

if (!globalThis.DOMMatrixReadOnly) {
  globalThis.DOMMatrixReadOnly = DOMMatrixReadOnlyMock as unknown as typeof DOMMatrixReadOnly;
}

function readInlinePixelDimension(value: string) {
  return value.endsWith('px') ? Number.parseFloat(value) : 0;
}

function elementInlineWidth(element: HTMLElement) {
  return readInlinePixelDimension(element.style.width);
}

function elementInlineHeight(element: HTMLElement) {
  return readInlinePixelDimension(element.style.height);
}

if (typeof HTMLElement !== 'undefined') {
  Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
    configurable: true,
    get() {
      return elementInlineWidth(this);
    },
  });

  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
    configurable: true,
    get() {
      return elementInlineHeight(this);
    },
  });

  const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;

  HTMLElement.prototype.getBoundingClientRect = function getBoundingClientRect() {
    const width = elementInlineWidth(this);
    const height = elementInlineHeight(this);
    if (width > 0 || height > 0) {
      return {
        bottom: height,
        height,
        left: 0,
        right: width,
        top: 0,
        width,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      };
    }

    return originalGetBoundingClientRect.call(this);
  };
}

let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;
let unexpectedConsoleCalls: UnexpectedConsoleCall[] = [];

function formatConsoleArgs(args: unknown[]) {
  return args
    .map((value) => (value instanceof Error ? value.stack ?? value.message : String(value)))
    .join(' ');
}

function recordUnexpectedConsole(method: ConsoleMethod, args: unknown[]) {
  unexpectedConsoleCalls.push({ method, args });
}

function throwUnexpectedConsoleCalls(calls: UnexpectedConsoleCall[]) {
  const formatted = calls
    .map((call) => `[unexpected console.${call.method}] ${formatConsoleArgs(call.args)}`)
    .join('\n');

  throw new Error([
    'Unexpected console calls detected during the test.',
    formatted,
  ].join('\n'));
}

beforeEach(() => {
  unexpectedConsoleCalls = [];

  consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation((...args) => {
    recordUnexpectedConsole('warn', args);
  });
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
    recordUnexpectedConsole('error', args);
  });
});

afterEach(() => {
  cleanup();

  const recordedCalls = unexpectedConsoleCalls;
  unexpectedConsoleCalls = [];

  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();

  if (recordedCalls.length) {
    throwUnexpectedConsoleCalls(recordedCalls);
  }
});
