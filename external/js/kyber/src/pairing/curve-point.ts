import BN from 'bn.js';
import GfP from './gfp';
import { p } from './constants';

type BNType = Buffer | string | number | BN;

const curveB = new GfP(new BN(3));

export default class CurvePoint {
    static generator = new CurvePoint(1, -2, 1, 0);
    
    private x: GfP;
    private y: GfP;
    private z: GfP;
    private t: GfP;

    constructor(x?: BNType, y?: BNType, z?: BNType, t?: BNType) {
        this.x = new GfP(x || 0);
        this.y = new GfP(y || 1);
        this.z = new GfP(z || 0);
        this.t = new GfP(t || 0);
    }

    getX(): GfP {
        return this.x;
    }

    getY(): GfP {
        return this.y;
    }

    isOnCurve(): boolean {
        let yy = this.y.sqr();
        const xxx = this.x.pow(new BN(3));

        yy = yy.sub(xxx);
        yy = yy.sub(curveB);
        if (yy.signum() < 0 || yy.compareTo(new GfP(p)) >= 0) {
            yy = yy.mod(p);
        }

        return yy.signum() == 0;
    }

    setInfinity(): void {
        this.x = new GfP(0);
        this.y = new GfP(1);
        this.z = new GfP(0);
        this.t = new GfP(0);
    }

    isInfinity(): boolean {
        return this.z.isZero();
    }

    add(a: CurvePoint, b: CurvePoint): void {
        if (a.isInfinity()) {
            this.copy(b)
            return;
        }

        if (b.isInfinity()) {
            this.copy(a);
            return;
        }

        const z1z1 = a.z.sqr().mod(p);
        const z2z2 = b.z.sqr().mod(p);
        const u1 = a.x.mul(z2z2).mod(p);
        const u2 = b.x.mul(z1z1).mod(p);

        let t = b.z.mul(z2z2).mod(p);
        const s1 = a.y.mul(t).mod(p);

        t = a.z.mul(z1z1).mod(p);
        const s2 = b.y.mul(t).mod(p);

        const h = u2.sub(u1);

        t = h.add(h);
        const i = t.sqr().mod(p);
        const j = h.mul(i).mod(p);

        t = s2.sub(s1);
        if (h.signum() === 0 && t.signum() === 0) {
            this.dbl(a);
            return;
        }

        const r = t.add(t);
        const v = u1.mul(i).mod(p);

        let t4 = r.sqr().mod(p);
        t = v.add(v);
        let t6 = t4.sub(j);
        this.x = t6.sub(t).mod(p);

        t = v.sub(this.x);
        t4 = s1.mul(j).mod(p);
        t6 = t4.add(t4);
        t4 = r.mul(t).mod(p);
        this.y = t4.sub(t6).mod(p);

        t = a.z.add(b.z);
        t4 = t.sqr().mod(p);
        t = t4.sub(z1z1);
        t4 = t.sub(z2z2);
        this.z = t4.mul(h).mod(p);
    }

    dbl(a: CurvePoint): void {
        const A = a.x.sqr().mod(p);
        const B = a.y.sqr().mod(p);
        const C = B.sqr().mod(p);

        let t = a.x.add(B);
        let t2 = t.sqr().mod(p);
        t = t2.sub(A);
        t2 = t.sub(C);
        const d = t2.add(t2);
        t = A.add(A);
        const e = t.add(A);
        const f = e.sqr().mod(p);

        t = d.add(d);
        this.x = f.sub(t).mod(p);

        t = C.add(C);
        t2 = t.add(t);
        t = t2.add(t2);
        this.y = d.sub(this.x);
        t2 = e.mul(this.y).mod(p);
        this.y = t2.sub(t).mod(p);

        t = a.y.mul(a.z).mod(p);
        this.z = t.add(t).mod(p);
    }

    mul(a: CurvePoint, scalar: BN): void {
        const sum = new CurvePoint();
        sum.setInfinity();
        const t = new CurvePoint();

        for (let i = scalar.bitLength(); i >= 0; i--) {
            t.dbl(sum);
            if (scalar.testn(i)) {
                sum.add(t, a);
            } else {
                sum.copy(t);
            }
        }

        this.copy(sum);
    }

    makeAffine(): void {
        if (this.z.isOne()) {
            return;
        } else if (this.z.isZero()) {
            this.setInfinity();
            return;
        }

        const zInv = this.z.invmod(p);
        let t = this.y.mul(zInv).mod(p);
        const zInv2 = zInv.sqr().mod(p);
        this.y = t.mul(zInv2).mod(p);
        t = this.x.mul(zInv2).mod(p);
        this.x = t;
        this.z = new GfP(1);
        this.t = new GfP(1);
    }

    negative(a: CurvePoint): void {
        this.x = a.x;
        this.y = a.y.negate();
        this.z = a.z;
        this.t = new GfP(0);
    }

    copy(p: CurvePoint): void {
        this.x = p.x.clone();
        this.y = p.y.clone();
        this.z = p.z.clone();
        this.t = p.t.clone();
    }

    clone(): CurvePoint {
        const p = new CurvePoint();
        p.copy(this);

        return p;
    }

    equals(o: any): o is CurvePoint {
        if (!(o instanceof CurvePoint)) {
            return false;
        }

        const a = this.clone();
        a.makeAffine();

        const b = o.clone();
        b.makeAffine();

        return a.x.equals(b.x) && a.y.equals(b.y) && a.z.equals(b.z) && a.t.equals(b.t);
    }

    toString(): string {
        const p = this.clone();
        p.makeAffine();

        return `(${p.getX().toString()},${p.getY().toString()})`;
    }
}
