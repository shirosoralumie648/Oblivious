import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, vi } from 'vitest';

type ConsoleMethod = 'warn' | 'error';

interface UnexpectedConsoleCall {
  method: ConsoleMethod;
  args: unknown[];
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
  const recordedCalls = unexpectedConsoleCalls;
  unexpectedConsoleCalls = [];

  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();

  if (recordedCalls.length) {
    throwUnexpectedConsoleCalls(recordedCalls);
  }
});
