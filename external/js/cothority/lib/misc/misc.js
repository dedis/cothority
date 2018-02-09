"use strict";

/**
 * Convert a byte buffer to a hexadecimal string.
 * @param {Uint8Array} buffer
 *
 * @throws {TypeError} when buffer is not Uint8Array
 * @returns {string} hexadecimal representation
 */
function uint8ArrayToHex(buffer) {
  if (buffer.constructor !== Uint8Array) throw new TypeError();

  return Array.from(buffer)
    .map((element, index) => {
      return ("00" + element.toString(16)).slice(-2);
    })
    .join("");
}

/**
 * Convert a hexadecimal string to a Uint8Array.
 * @param {string} hex
 *
 * @throws {TypeError} when hex is not a string
 * @return {Uint8Array} byte buffer
 */
function hexToUint8Array(hex) {
  if (typeof hex !== "string") throw new TypeError();

  return new Uint8Array(Math.ceil(hex.length / 2)).map((element, index) => {
    return parseInt(hex.substr(index * 2, 2), 16);
  });
}

/**
 * Compares two Uint8Array buffers. If constant is true, it does the comparison in constant time.
 *
 * @param {Uint8Array} arr1 first buffer
 * @param {Uint8Array} arr2 second buffer
 * @param {Boolean} constant if true, constant time comparison is done.
 * @returns {Boolean} True if both buffers are equals, false otherwise.
 */
function uint8ArrayCompare(arr1, arr2, constant) {
  if (arr1.constructor !== Uint8Array) throw TypeError;
  if (arr2.constructor !== Uint8Array) throw TypeError;

  if (arr1.length != arr2.length) {
    return false;
  }
  if (constant) {
    return constantCompare(arr1, arr2);
  } else {
    return normalCompare(arr1, arr2);
  }
}

function normalCompare(arr1, arr2) {
  for (var i = 0; i < arr1.length; i++) {
    if (arr1[i] != arr2[i]) {
      return false;
    }
  }
  return true;
}

function constantCompare(arr1, arr2) {
  throw new Error("not implemented yet");
}

function getBitmaskLength(bitmask) {
  if (bitmask.constructor !== Uint8Array) throw TypeError;

  const masklen = bitmask.length;
  return masklen << 3;
}

/**
 * getSetBits returns an array of indices for each bit set in the given bitmask. See cosi bitmask for more information.
 *
 * @param {Uint8Array} mask the bit mask
 * @returns {Array} array of indices of bits sets
 */
const BN = require("bn.js");
function getSetBits(mask) {
  if (mask.constructor !== Uint8Array) throw TypeError;

  const masklen = mask.length;
  const n = masklen << 3;
  const indices = [];
  for (var i = 0; i < n; i++) {
    var byt = i >> 3;
    var bit = 1 << (i & 7);
    if (byt < masklen && (mask[byt] & bit) != 0) {
      // Participant i disabled
      // take the idx relative to the current byte
      var idx = i % 8;
      // since we're reading bits in a byte from the lowest bit first,
      // need to reverse
      idx = (byt << 3) + (7 - idx);
      indices.push(idx);
    }
  }
  return indices.sort((a, b) => a - b);
}
/** @module misc */
exports.uint8ArrayToHex = uint8ArrayToHex;
exports.hexToUint8Array = hexToUint8Array;
exports.uint8ArrayCompare = uint8ArrayCompare;
exports.getSetBits = getSetBits;
exports.getBitmaskLength = getBitmaskLength;
