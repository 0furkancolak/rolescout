// The single source of the design tokens. index.css repeats these values (a
// test keeps the two in step) and tests check their contrast.

export const tokens = {
  light: {
    canvas: "#f5f1ea",
    surface: "#fffdf9",
    raised: "#ffffff",
    ink: "#17171a",
    "ink-muted": "#625d55",
    "ink-inverse": "#fffdf9",
    line: "#e6dfd3",
    "line-strong": "#8a8171",
    sage: "#a9cdb4",
    "sage-deep": "#35604a",
    amber: "#f0c777",
    "amber-deep": "#8a5a06",
    lavender: "#cbc3f2",
    "lavender-deep": "#4e4390",
    danger: "#b4412f",
    "danger-soft": "#f6ddd7",
    focus: "#4e4390",
  },
  dark: {
    canvas: "#121110",
    surface: "#1b1a18",
    raised: "#262421",
    ink: "#f3eee6",
    "ink-muted": "#a8a096",
    "ink-inverse": "#17171a",
    line: "#34312c",
    "line-strong": "#8a8379",
    sage: "#2f4a3a",
    "sage-deep": "#9fd0ae",
    amber: "#4f3b12",
    "amber-deep": "#f0c777",
    lavender: "#3d3768",
    "lavender-deep": "#cbc3f2",
    danger: "#f09a8a",
    "danger-soft": "#3a211c",
    focus: "#cbc3f2",
  },
} as const;

export type TokenName = keyof (typeof tokens)["light"];
