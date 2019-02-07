"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
/**
 * Masks are used alongside with aggregated signatures to announce which peer
 * has actually signed the message. It's essentially a bit mask.
 */
class Mask {
    constructor(publics, mask) {
        if (publics.length === 0 || publics.length << 3 < mask.length - 1) {
            throw new Error("length of the public keys and the mask don't match");
        }
        this._aggregate = publics[0].clone().null();
        for (let i = 0; i < publics.length; i++) {
            const k = i >> 3;
            const msk = 1 << (i & 7);
            if ((mask[k] & msk) !== 0) {
                this._aggregate.add(this._aggregate, publics[i]);
            }
            else {
                const neg = publics[i].clone().neg(publics[i]);
                this._aggregate.add(this._aggregate, neg);
            }
        }
    }
    get aggregate() {
        return this._aggregate;
    }
}
exports.default = Mask;
