/// <reference types="node" />
import BN from "bn.js";
/**
 * bits choses a random buffer with a maximum bitlength
 * If exact is `true`, chose a buffer with *exactly* that bitlenght not less
 * @param bitlen    maximum size of the resulting buffer
 * @param exact     when true the buffer has the given length
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
export declare function bits(bitlen: number, exact: boolean, callback: (length: number) => Buffer): Buffer;
/**
 * int choses a random uniform buffer less than given modulus
 * @param mod       modulus
 * @param callback  buffer generator function
 * @returns         randomly filled buffer
 */
export declare function int(mod: BN, callback: (length: number) => Buffer): Buffer;
