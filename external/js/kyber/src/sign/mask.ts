import { Point } from '../.';

/**
 * Masks are used alongside with aggregated signatures to announce which peer
 * has actually signed the message. It's essentially a bit mask.
 */
export default class Mask {
    private _aggregate: Point;

    constructor(publics: Point[], mask: Buffer) {
        if (publics.length === 0 || publics.length <= (mask.length-1) * 8) {
            throw new Error("length of the public keys and the mask don't match");
        }

        this._aggregate = publics[0].clone().null();

        for (let i = 0; i < publics.length; i++) {
            const k = i >> 3;
            const msk = 1 << (i & 7);

            if ((mask[k] & msk) !== 0) {
                this._aggregate.add(this._aggregate, publics[i]);
            } else {
                const neg = publics[i].clone().neg(publics[i]);
                this._aggregate.add(this._aggregate, neg);
            }
        }
    }

    get aggregate(): Point {
        return this._aggregate;
    }
}
