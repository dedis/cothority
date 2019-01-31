/// <reference types="node" />
import { Scalar, Point } from "../..";
export declare const Suite: import("../..").Group;
export declare class RingSig {
    readonly c0: Scalar;
    readonly s: Scalar[];
    readonly tag: Point;
    constructor(c0: Scalar, s: Scalar[], tag?: Point);
    encode(): Buffer;
    static fromBytes(signatureBuffer: Buffer, isLinkableSig: boolean): RingSig;
}
export declare function sign(message: Buffer, anonymitySet: Point[], secret: Scalar, linkScope?: Buffer): RingSig;
export declare function verify(message: Buffer, anonymitySet: Point[], signatureBuffer: Buffer, linkScope?: Buffer): boolean;
