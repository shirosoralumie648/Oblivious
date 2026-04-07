import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, vi } from 'vitest';

let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

function formatConsoleArgs(args: unknown[]) {
  return args
    .map((value) => (value instanceof Error ? value.stack ?? value.message : String(value)))
    .join(' ');
}

function failUnexpectedConsole(method: 'warn' | 'error', args: unknown[]) {
  throw new Error(`[unexpected console.${method}] ${formatConsoleArgs(args)}`);
}

beforeEach(() => {
  consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation((...args) => {
    failUnexpectedConsole('warn', args);
  });
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
    failUnexpectedConsole('error', args);
  });
});

afterEach(() => {
  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();
});
