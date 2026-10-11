// Sizes shared by layout, routing and drawing (ticket 016). No imports, so
// layout and routing can run outside the browser (tests, the layout harness).
export const CARD_W = 200;
export const CARD_H = 72;
export const GROUP_PAD = 24;
export const GROUP_HEAD = 36;

export type Box = { x: number; y: number; w: number; h: number };
