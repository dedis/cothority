import BN from "bn.js";
import { zeroBN } from "./constants";

/**
 * bits choses a random buffer with a maximum bitlength
 * If exact is `true`, chose a buffer with *exactly* that bitlenght not less
 * @param bitlen    maximum size of the resulting buffer
 * @param exact     when true the buffer has the given length
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
export function bits(bitlen: number, exact: boolean, callback: (length: number) => Buffer): Buffer {
    let b = callback((bitlen + 7) >> 3);
    let highbits = bitlen & 7;
    if (highbits != 0) {
        b[0] &= ~(0xff << highbits);
    }
    
    if (exact) {
        if (highbits !== 0) {
            b[0] |= 1 << (highbits - 1);
        } else {
            b[0] |= 0x80;
        }
    }
    return Buffer.from(b);
}

/**
 * int choses a random uniform buffer less than given modulus
 * @param mod       modulus
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
export function int(mod: BN, callback: (length: number) => Buffer): Buffer {
    let bitlength = mod.bitLength();
    let i;
    while (true) {
        const bytes = bits(bitlength, false, callback);
        i = new BN(bytes);
        if (i.cmp(zeroBN) > 0 && i.cmp(mod) < 0) {
            return bytes;
        }
    }
}