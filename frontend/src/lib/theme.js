export const dracula = {
  bg:         '#282a36',
  currentLine:'#44475a',
  fg:         '#f8f8f2',
  comment:    '#6272a4',
  green:      '#50fa7b',
  red:        '#ff5555',
  orange:     '#ffb86c',
  cyan:       '#8be9fd',
  purple:     '#bd93f9',
  pink:       '#ff79c6',
  yellow:     '#f1fa8c',
};

// Semantic aliases for use in components
export const theme = {
  bg:         dracula.bg,
  bgCard:     dracula.currentLine,
  text:       dracula.fg,
  textMuted:  dracula.comment,
  success:    dracula.green,
  error:      dracula.red,
  warning:    dracula.orange,
  info:       dracula.cyan,
  accent:     dracula.purple,
  accentAlt:  dracula.pink,
};
