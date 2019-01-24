import BN from "bn.js";

export type BNType = number | string | number[] | Buffer | BN;

export const zeroBN = new BN(0);
