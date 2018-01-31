"use strict";

/**
 * Class PRNG defines a PRNG using a Linear Congruent Generator
 * Javascript doesn't have a seedable PRNG in stdlib and this
 * implementation is to be used for deterministic outputs in tests
 */
class PRNG {
  constructor(seed) {
    this.seed = seed;
  }

  setSeed(seed) {
    this.seed = seed;
  }

  genByte() {
    this.seed = (this.seed * 9301 + 49297) % 233280;
    let rnd = this.seed / 233280;

    return Math.floor(rnd * 255);
  }

  pseudoRandomBytes(n) {
    let arr = new Uint8Array(n);
    for (let i = 0; i < n; i++) {
      arr[i] = this.genByte();
    }
    return arr;
  }
}

module.exports = {
  PRNG
};
