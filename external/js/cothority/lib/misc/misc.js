'use strict';

/** @module misc */

exports.uint8ArrayToHex = uint8ArrayToHex;
exports.hexToUint8Array = hexToUint8Array;
exports.reverseHex = reverseHex;

/**
 * Convert a byte buffer to a hexadecimal string.
 * @param {Uint8Array} buffer
 *
 * @throws {TypeError} when buffer is not Uint8Array
 * @returns {string} hexadecimal representation
 */
function uint8ArrayToHex(buffer) {
    if (buffer.constructor !== Uint8Array)
	throw new TypeError;

    return Array.from(buffer).map((element, index) => {
	return ('00' + element.toString(16)).slice(-2);
    }).join('');
}

/**
 * Convert a hexadecimal string to a Uint8Array.
 * @param {string} hex
 *
 * @throws {TypeError} when hex is not a string
 * @return {Uint8Array} byte buffer
 */
function hexToUint8Array(hex) {
    if (typeof hex !== 'string')
	throw new TypeError;

    return new Uint8Array(Math.ceil(hex.length / 2)).map((element, index) => {
	return parseInt(hex.substr(index * 2, 2), 16);
    });
}

/**
 * Reverse a hexadecimal string.
 * @param {string} hex
 *
 * @throws {TypeError} when hex is not a string
 * @return {string} reversed hex string
 */
function reverseHex(hex) {
    if (typeof hex !== 'string')
	throw new TypeError;

    let reversed = '';
    for (let i = 0; i < hex.length-1; i += 2)
	reversed = hex.substring(i, i+2) + reversed;

    if (hex.length % 2 === 1)
	reversed = hex[hex.length-1] + reversed;

    return reversed;
}
