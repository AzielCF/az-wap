---
trigger: always_on
---

Always build your visual components inside assets/main.css, and base your styling on DaisyUI classes to keep a consistent design system.
Any reusable component styles must be defined in main.css.

You must never use arbitrary pixel values like text-[XXpx].
Always use the standard, predefined size utilities instead — do not hardcode sizes.