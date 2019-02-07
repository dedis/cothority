/// <reference types="node" />
import { Point } from '../.';
/**
 * Masks are used alongside with aggregated signatures to announce which peer
 * has actually signed the message. It's essentially a bit mask.
 */
export default class Mask {
    private _aggregate;
    constructor(publics: Point[], mask: Buffer);
    readonly aggregate: Point;
}
