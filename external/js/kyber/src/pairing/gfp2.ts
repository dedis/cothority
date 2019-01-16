import BN from 'bn.js';
import GfP from './gfp';
import { p } from './constants';

type BNType = Buffer | string | number | BN;

export default class GfP2 {
    static zero(): GfP2 {
        return new GfP2(0, 0);
    }

    static one(): GfP2 {
        return new GfP2(0, 1);
    }

    private x: GfP;
    private y: GfP;

    constructor(x: BNType | GfP, y: BNType | GfP) {
        this.x = x instanceof GfP ? x : new GfP(x || 0);
        this.y = y instanceof GfP ? y : new GfP(y || 0);
    }

    getX(): GfP {
        return this.x;
    }

    getY(): GfP {
        return this.y;
    }

    isZero(): boolean {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(0);
    }

    isOne(): boolean {
        return this.x.getValue().eqn(0) && this.y.getValue().eqn(1);
    }

    conjugate(): GfP2 {
        return new GfP2(this.x.negate(), this.y);
    }

    negative(): GfP2 {
        return new GfP2(this.x.negate(), this.y.negate());
    }

    add(a: GfP2): GfP2 {
        const x = this.x.add(a.x).mod(p);
        const y = this.y.add(a.y).mod(p);
        return new GfP2(x, y);
    }

    sub(a: GfP2): GfP2 {
        const x = this.x.sub(a.x).mod(p);
        const y = this.y.sub(a.y).mod(p);
        return new GfP2(x, y);
    }

    mul(a: GfP2): GfP2 {
        let tx = this.x.mul(a.y);
        let t = a.x.mul(this.y);
        tx = tx.add(t).mod(p);

        let ty = this.y.mul(a.y).mod(p);
        t = this.x.mul(a.x).mod(p);
        ty = ty.sub(t).mod(p);

        return new GfP2(tx, ty);
    }

    mulScalar(k: GfP): GfP2 {
        const x = this.x.mul(k);
        const y = this.y.mul(k);

        return new GfP2(x, y);
    }

    mulXi(): GfP2 {
        let tx = this.x.add(this.x);
        tx = tx.add(this.x);
        tx = tx.add(this.y);

        let ty = this.y.add(this.y);
        ty = ty.add(this.y);
        ty = ty.sub(this.x);

        return new GfP2(tx, ty);
    }

    square(): GfP2 {
        const t1 = this.y.sub(this.x);
        const t2 = this.x.add(this.y);

        const ty = t1.mul(t2).mod(p);
        // intermediate modulo is due to a missing implementation
        // in the library that is actually using the unsigned left
        // shift any time
        const tx = this.x.mul(this.y).mod(p).shiftLeft(1).mod(p);

        return new GfP2(tx, ty);
    }

    invert(): GfP2 {
        let t = this.y.mul(this.y);
        let t2 = this.x.mul(this.x);
        t = t.add(t2);

        const inv = t.invmod(p);
        const tx = this.x.negate().mul(inv).mod(p);
        const ty = this.y.mul(inv).mod(p);

        return new GfP2(tx, ty);
    }

    equals(o: any): o is GfP2 {
        return this.x.equals(o.x) && this.y.equals(o.y);
    }

    clone(): GfP2 {
        return new GfP2(this.x, this.y);
    }

    toString(): string {
        return `(${this.x.toHex()},${this.y.toHex()})`;
    }
}
