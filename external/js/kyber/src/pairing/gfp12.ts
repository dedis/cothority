import BN from 'bn.js';
import GfP6 from './gfp6';
import GfP from './gfp';
import { xiToPMinus1Over6, xiToPSquaredMinus1Over6, xiToPSquaredMinus1Over3 } from './constants';

export default class GfP12 {
    static zero(): GfP12 {
        return new GfP12(GfP6.zero(), GfP6.zero());
    }

    static one(): GfP12 {
        return new GfP12(GfP6.zero(), GfP6.one());
    }

    private x: GfP6;
    private y: GfP6;

    constructor(x?: GfP6, y?: GfP6) {
        this.x = x || GfP6.zero();
        this.y = y || GfP6.zero();
    }

    getX(): GfP6 {
        return this.x;
    }

    getY(): GfP6 {
        return this.y;
    }

    isZero(): boolean {
        return this.x.isZero() && this.y.isZero();
    }

    isOne(): boolean {
        return this.x.isZero() && this.y.isOne();
    }

    conjugate(): GfP12 {
        const x = this.x.neg();

        return new GfP12(x, this.y);
    }

    neg(): GfP12 {
        const x = this.x.neg();
        const y = this.y.neg();

        return new GfP12(x, y);
    }

    frobenius(): GfP12 {
        const x = this.x.frobenius().mulScalar(xiToPMinus1Over6);
        const y = this.y.frobenius();
        return new GfP12(x, y);
    }

    frobeniusP2(): GfP12 {
        const x = this.x.frobeniusP2().mulGfP(new GfP(xiToPSquaredMinus1Over6));
        const y = this.y.frobeniusP2();
        return new GfP12(x, y);
    }

    add(b: GfP12): GfP12 {
        const x = this.x.add(b.x);
        const y = this.y.add(b.y);
        return new GfP12(x, y);
    }

    sub(b: GfP12): GfP12 {
        const x = this.x.sub(b.x);
        const y = this.y.sub(b.y);
        return new GfP12(x, y);
    }

    mul(b: GfP12): GfP12 {
        const x = this.x.mul(b.y)
            .add(b.x.mul(this.y));

        const y = this.y.mul(b.y)
            .add(this.x.mul(b.x).mulTau());

        return new GfP12(x, y);
    }

    mulScalar(k: GfP6): GfP12 {
        const x = this.x.mul(k);
        const y = this.y.mul(k);
        return new GfP12(x, y);
    }

    exp(power: BN): GfP12 {
        let sum = GfP12.one();
        let t : GfP12;

        for (let i = power.bitLength() - 1; i >= 0; i--) {
            t = sum.square();
            if (power.testn(i)) {
                sum = t.mul(this);
            } else {
                sum = t;
            }
        }

        return sum;
    }

    square(): GfP12 {
        const v0 = this.x.mul(this.y);

        let t = this.x.mulTau();
        t = this.y.add(t);
        let ty = this.x.add(this.y);
        ty = ty.mul(t).sub(v0);
        t = v0.mulTau();
        ty = ty.sub(t);

        return new GfP12(v0.add(v0), ty);
    }

    invert(): GfP12 {
        let t1 = this.x.square();
        let t2 = this.y.square();
        t1 = t2.sub(t1.mulTau());
        t2 = t1.invert();

        return new GfP12(this.x.neg(), this.y).mulScalar(t2);
    }

    equals(o: any): o is GfP12 {
        return this.x.equals(o.x) && this.y.equals(o.y);
    }

    toString(): string {
        return `(${this.x.toString()}, ${this.y.toString()})`;
    }
}
