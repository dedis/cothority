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

const unhexlify = function(str) {
  let result = new Uint8Array(str.length >> 1);
  for (let c = 0, i = 0, l = str.length; i < l; i += 2, c++) {
    result[c] = parseInt(str.substr(i, 2), 16);
  }
  return result;
};

const hexToUint8Array = hex => {
  let bytes = new Uint8Array(hex.length >> 1);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(hex.substr(i << 1, 2), 16);
  }
  return bytes;
};

module.exports = {
  PRNG,
  unhexlify,
  hexToUint8Array
};
