"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const bn_js_1 = __importDefault(require("bn.js"));
const constants_1 = __importDefault(require("./constants"));
/**
 * bits choses a random buffer with a maximum bitlength
 * If exact is `true`, chose a buffer with *exactly* that bitlenght not less
 * @param bitlen    maximum size of the resulting buffer
 * @param exact     when true the buffer has the given length
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
function bits(bitlen, exact, callback) {
    let b = callback((bitlen + 7) >> 3);
    let highbits = bitlen & 7;
    if (highbits != 0) {
        b[0] &= ~(0xff << highbits);
    }
    if (exact) {
        if (highbits !== 0) {
            b[0] |= 1 << (highbits - 1);
        }
        else {
            b[0] |= 0x80;
        }
    }
    return Buffer.from(b);
}
exports.bits = bits;
/**
 * int choses a random uniform buffer less than given modulus
 * @param mod       modulus
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
function int(mod, callback) {
    let bitlength = mod.bitLength();
    let i;
    while (true) {
        const bytes = bits(bitlength, false, callback);
        i = new bn_js_1.default(bytes);
        if (i.cmp(constants_1.default.zeroBN) > 0 && i.cmp(mod) < 0) {
            return bytes;
        }
    }
}
exports.int = int;
