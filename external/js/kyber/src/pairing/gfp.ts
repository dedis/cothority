import BN from 'bn.js';

type BNType = Buffer | string | number | BN;

export default class GfP {
    private static ELEM_SIZE = 256 / 8;

    private v: BN;

    constructor(value: BNType) {
        this.v = new BN(value);
    }

    getValue(): BN {
        return this.v;
    }

    signum(): -1 | 0 | 1 {
        return this.v.cmpn(0);
    }

    isOne(): boolean {
        return this.v.eq(new BN(1));
    }

    isZero(): boolean {
        return this.v.isZero();
    }

    add(a: GfP): GfP {
        return new GfP(this.v.add(a.v));
    }

    sub(a: GfP): GfP {
        return new GfP(this.v.sub(a.v));
    }

    mul(a: GfP): GfP {
        return new GfP(this.v.mul(a.v));
    }

    sqr(): GfP {
        return new GfP(this.v.sqr());
    }

    pow(k: BN) {
        return new GfP(this.v.pow(k));
    }

    mod(p: BN): GfP {
        return new GfP(this.v.umod(p));
    }

    invmod(p: BN): GfP {
        return new GfP(this.v.invm(p));
    }

    negate(): GfP {
        return new GfP(this.v.neg());
    }

    shiftLeft(k: number): GfP {
        return new GfP(this.v.shln(k));
    }

    compareTo(o: any): 0 | -1 | 1 {
        return this.v.cmp(o.v);
    }

    clone(): GfP {
        return new GfP(this.v.clone());
    }

    equals(o: any): o is GfP {
        return this.v.eq(o.v);
    }

    toBytes(): Buffer {
        return this.v.toArrayLike(Buffer, 'be', GfP.ELEM_SIZE);
    }

    toString(): string {
        return this.toHex();
    }

    toHex(): string {
        return this.v.toString('hex');
    }
}
