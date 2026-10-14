// Log-bucketed latency histograms (DDSketch-style): bucket i covers
// [MIN·γ^i, MIN·γ^(i+1)), so any percentile read from one is within ±1% of the
// true value. Percentiles over a window come from summing histograms, never
// from averaging percentiles.

export const GAMMA = 1.02;
export const MIN_MS = 0.05;
export const MAX_MS = 120_000;
const LN_G = Math.log(GAMMA);
export const BUCKETS = Math.ceil(Math.log(MAX_MS / MIN_MS) / LN_G) + 1; // 743

export function bucketOf(ms: number): number {
  if (ms <= MIN_MS) return 0;
  const i = Math.floor(Math.log(ms / MIN_MS) / LN_G);
  return i >= BUCKETS ? BUCKETS - 1 : i;
}

/** Midpoint of a bucket, in ms. */
export function valueOf(i: number): number {
  return MIN_MS * Math.pow(GAMMA, i + 0.5);
}

/**
 * Reads several quantiles (ascending) from histogram `h` at offset `off` in one pass.
 * Writes NaN when the histogram is empty.
 */
export function quantiles(h: Float32Array | Float64Array, off: number, qs: number[], out: Float32Array, outOff: number): number {
  let total = 0;
  for (let i = 0; i < BUCKETS; i++) total += h[off + i];
  if (total <= 0) {
    for (let k = 0; k < qs.length; k++) out[outOff + k] = NaN;
    return 0;
  }
  let k = 0;
  let target = qs[0] * total;
  let acc = 0;
  for (let i = 0; i < BUCKETS && k < qs.length; i++) {
    acc += h[off + i];
    while (k < qs.length && acc >= target) {
      out[outOff + k] = valueOf(i);
      k++;
      if (k < qs.length) target = qs[k] * total;
    }
  }
  for (; k < qs.length; k++) out[outOff + k] = valueOf(BUCKETS - 1);
  return total;
}
