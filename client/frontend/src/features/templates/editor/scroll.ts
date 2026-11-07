// Smooth scrolling moves the whole page, which is exactly what "reduce motion" asks to avoid.

/** How to scroll a field into view: smoothly, unless the system asks for reduced motion. */
export function scrollBehavior(): ScrollBehavior {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth';
}
