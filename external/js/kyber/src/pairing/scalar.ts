import { randomBytes } from 'crypto';
import BN from 'bn.js';
import { Scalar } from '../index';
import { p } from './constants';
import { int } from '../random';

export type BNType = number | string | number[] | Buffer | BN;

/**
 * Scalar used in combination with G1 and G2 points
 */
export default class BN256Scalar implements Scalar {
    private v: BN;

    constructor(value?: BNType) {
        this.v = new BN(value).umod(p);
    }

    /**
     * Get the BigNumber value of the scalar
     * @returns the value
     */
    getValue(): BN {
        return this.v;
    }

    /** @inheritdoc */
    set(a: BN256Scalar): BN256Scalar {
        this.v = a.v.clone();
        return this;
    }

    /** @inheritdoc */
    one(): BN256Scalar {
        this.v = new BN(1);
        return this;
    }

    /** @inheritdoc */
    zero(): BN256Scalar {
        this.v = new BN(0);
        return this;
    }

    /** @inheritdoc */
    add(a: BN256Scalar, b: BN256Scalar): BN256Scalar {
        this.v = a.v.add(b.v).umod(p);
        return this;
    }

    /** @inheritdoc */
    sub(a: BN256Scalar, b: BN256Scalar): BN256Scalar {
        this.v = a.v.sub(b.v).umod(p);
        return this;
    }

    /** @inheritdoc */
    neg(a: BN256Scalar): BN256Scalar {
        this.v = a.v.neg().umod(p);
        return this;
    }

    /** @inheritdoc */
    div(a: BN256Scalar, b: BN256Scalar): BN256Scalar {
        this.v = a.v.div(b.v).umod(p);
        return this;
    }

    /** @inheritdoc */
    mul(s1: BN256Scalar, b: BN256Scalar): BN256Scalar {
        this.v = s1.v.mul(b.v).umod(p);
        return this;
    }

    /** @inheritdoc */
    inv(a: BN256Scalar): BN256Scalar {
        this.v = a.v.invm(p);
        return this;
    }

    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256Scalar {
        callback = callback || randomBytes;
        
        const bytes = int(p, callback);
        this.setBytes(bytes);
        return this;
    }

    /** @inheritdoc */
    setBytes(bytes: Buffer): BN256Scalar {
        this.v = new BN(bytes, 16);
        return this;
    }

    /** @inheritdoc */
    marshalBinary(): Buffer {
        return this.v.toArrayLike(Buffer, 'be', 32);
    }

    /** @inheritdoc */
    unmarshalBinary(buf: Buffer | string): void {
        this.v = new BN(buf, 16);
    }

    /** @inheritdoc */
    marshalSize(): number{
        return 32;
    }

    /** @inheritdoc */
    clone(): BN256Scalar {
        const s = new BN256Scalar(new BN(this.v));
        return s;
    }

    /** @inheritdoc */
    equals(s2: BN256Scalar): boolean {
        return this.v.eq(s2.v);
    }
}
