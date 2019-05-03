import { Point } from '../.';

/**
 * Masks are used alongside with aggregated signatures to announce which peer
 * has actually signed the message. It's essentially a bit mask.
 */
export default class Mask {
    readonly publics: Point[];
    readonly aggregate: Point;

    private mask: Buffer;

    constructor(publics: Point[], mask: Buffer) {
        if (publics.length === 0 || publics.length <= (mask.length - 1) * 8) {
            throw new Error("length of the public keys and the mask don't match");
        }

        this.publics = publics.slice();
        this.mask = Buffer.from(mask);
        this.aggregate = publics[0].clone().null();

        for (let i = 0; i < publics.length; i++) {
            const k = i >> 3;
            const msk = 1 << (i & 7);

            if ((mask[k] & msk) !== 0) {
                this.aggregate.add(this.aggregate, publics[i]);
            } else {
                const neg = publics[i].clone().neg(publics[i]);
                this.aggregate.add(this.aggregate, neg);
            }
        }
    }

    /**
     * Return the number of participants, in other words the number of 1s in the mask
     * 
     * @return the number of participants
     */
    getCountEnabled(): number {
        let hw = 0;
        for (let i = 0; i < this.publics.length; i++) {
            const k = i >> 3;
            const msk = 1 << (i & 7);
            if ((this.mask[k] & msk) !== 0) {
                hw++;
            }
        }

        return hw;
    }

    /**
     * Return the total number of public keys assigned to the mask
     * 
     * @return the total number of public keys
     */
    getCountTotal(): number {
        return this.publics.length;
    }

    /**
     * Return true if the bit at the given index is enabled
     * 
     * @param i The index
     * @return true if the bit is enabled, false otherwise
     */
    isIndexEnabled(i: number): boolean {
        if (i < 0 || i >= this.publics.length) {
            throw new Error('index out of bound');
        }

        const k = i >> 3;
        const msk = 1 << (i & 7);
        return (this.mask[k] & msk) !== 0;
    }
}
