import { useEffect, useState } from 'react';

/**
 * `value` once it has stopped changing for `delay` ms. For live regions: a count that changes on
 * every keystroke is announced once the user pauses, not on each key.
 */
export function useSettled<T>(value: T, delay: number): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return settled;
}
