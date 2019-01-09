/// <reference types="node" />
import BN = require("bn.js");
/**
* bits choses a random buffer with a maximum bitlength
* If exact is `true`, chose a buffer with *exactly* that bitlenght not less
*/
export declare function bits(bitlen: number, exact: boolean, callback: (length: number) => Buffer): Buffer;
/**
* int choses a random uniform buffer less than given modulus
*/
export declare function int(mod: BN, callback: (length: number) => Buffer): Buffer;
