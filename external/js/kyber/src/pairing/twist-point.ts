import BN from 'bn.js';
import GfP2 from './gfp2';

const twistB = new GfP2(
    '6500054969564660373279643874235990574282535810762300357187714502686418407178',
    '45500384786952622612957507119651934019977750675336102500314001518804928850249'
);

/**
 * Point class used by G2
 */
export default class TwistPoint {
    static generator = new TwistPoint(
        new GfP2(
            '21167961636542580255011770066570541300993051739349375019639421053990175267184',
            '64746500191241794695844075326670126197795977525365406531717464316923369116492',
        ),
        new GfP2(
            '20666913350058776956210519119118544732556678129809273996262322366050359951122',
            '17778617556404439934652658462602675281523610326338642107814333856843981424549',
        ),
        new GfP2('0', '1'),
        new GfP2('0', '1'),
    );

    private x: GfP2;
    private y: GfP2;
    private z: GfP2;
    private t: GfP2;

    constructor(x?: GfP2, y?: GfP2, z?: GfP2, t?: GfP2) {
        this.x = x || GfP2.zero();
        this.y = y || GfP2.zero();
        this.z = z || GfP2.zero();
        this.t = t || GfP2.zero();
    }

    /**
     * Get the x element of the point
     * @returns the x element
     */
    getX(): GfP2 {
        return this.x;
    }

    /**
     * Get the y element of the point
     * @returns the y element
     */
    getY(): GfP2 {
        return this.y;
    }

    /**
     * Get the z element of the point
     * @returns the z element
     */
    getZ(): GfP2 {
        return this.z;
    }

    /**
     * Get the t element of the point
     * @returns the t element
     */
    getT(): GfP2 {
        return this.t;
    }

    /**
     * Check if the point is on the curve, meaning it's a valid point
     * @returns true for a valid point, false otherwise
     */
    isOnCurve(): boolean {
        const cpy = this.clone();
        cpy.makeAffine();
        if (cpy.isInfinity()) {
            return true;
        }

        const yy = cpy.y.square();
        const xxx = cpy.x.square().mul(cpy.x).add(twistB);

        return yy.equals(xxx);
    }

    /**
     * Set the point to the infinity value
     */
    setInfinity(): void {
        this.x = GfP2.zero();
        this.y = GfP2.one();
        this.z = GfP2.zero();
        this.t = GfP2.zero();
    }

    /**
     * Check if the point is the infinity
     * @returns true when the infinity, false otherwise
     */
    isInfinity(): boolean {
        return this.z.isZero();
    }

    /**
     * Add a to b and set the value to the point
     * @param a first point
     * @param b second point
     */
    add(a: TwistPoint, b: TwistPoint): void {
        if (a.isInfinity()) {
            this.copy(b);
            return;
        }
        if (b.isInfinity()) {
            this.copy(a);
            return;
        }

        const z12 = a.z.square();
        const z22 = b.z.square();
        const u1 = a.x.mul(z22);
        const u2 = b.x.mul(z12);

        let t = b.z.mul(z22);
        const s1 = a.y.mul(t);

        t = a.z.mul(z12);
        const s2 = b.y.mul(t);

        const h = u2.sub(u1);
        
        t = h.add(h);
        const i = t.square();
        const j = h.mul(i);

        t = s2.sub(s1);
        if (h.isZero() && t.isZero()) {
            this.double(a);
            return;
        }

        const r = t.add(t);
        const v = u1.mul(i);

        let t4 = r.square();
        t = v.add(v);
        let t6 = t4.sub(j);
        this.x = t6.sub(t);

        t = v.sub(this.x);
        t4 = s1.mul(j);
        t6 = t4.add(t4);
        t4 = r.mul(t);
        this.y = t4.sub(t6);

        t = a.z.add(b.z);
        t4 = t.square();
        t = t4.sub(z12);
        t4 = t.sub(z22);
        this.z = t4.mul(h);
    }

    /**
     * Compute the double of the given point and set the value
     * @param a the point
     */
    double(a: TwistPoint): void {
        const A = a.x.square();
        const B = a.y.square();
        const C = B.square();

        let t = a.x.add(B);
        let t2 = t.square();
        t = t2.sub(A);
        t2 = t.sub(C);
        const d = t2.add(t2);
        t = A.add(A);
        const e = t.add(A);
        const f = e.square();

        t = d.add(d);
        this.x = f.sub(t);

        t = C.add(C);
        t2 = t.add(t);
        t = t2.add(t2);
        this.y = d.sub(this.x);
        t2 = e.mul(this.y);
        this.y = t2.sub(t);

        t = a.y.mul(a.z);
        this.z = t.add(t);
    }

    /**
     * Multiply a point by a scalar and set the value to the point
     * @param a the point
     * @param k the scalar
     */
    mul(a: TwistPoint, k: BN): void {
        const sum = new TwistPoint();
        const t = new TwistPoint();

        for (let i = k.bitLength(); i >= 0; i--) {
            t.double(sum);
            if (k.testn(i)) {
                sum.add(t, a);
            } else {
                sum.copy(t);
            }
        }

        this.copy(sum);
    }

    /**
     * Normalize the point coordinates
     */
    makeAffine(): void {
        if (this.z.isOne()) {
            return;
        } else if (this.z.isZero()) {
            this.setInfinity();
            return;
        }

        const zInv = this.z.invert();
        let t = this.y.mul(zInv);
        const zInv2 = zInv.square();
        this.y = t.mul(zInv2);
        this.x = this.x.mul(zInv2);
        this.z = GfP2.one();
        this.t = GfP2.one();
    }

    /**
     * Compute the negative of a and set the value to the point
     * @param a the point
     */
    neg(a: TwistPoint): void {
        this.x = a.x;
        this.y = a.y.negative();
        this.z = a.z;
    }

    /**
     * Fill the point with the values of a
     * @param a the point
     */
    copy(a: TwistPoint): void {
        this.x = a.x;
        this.y = a.y;
        this.z = a.z;
        this.t = a.t;
    }

    /**
     * Get the a clone of the current point
     * @returns a copy of the point
     */
    clone(): TwistPoint {
        return new TwistPoint(this.x, this.y, this.z, this.t);
    }

    /**
     * Check the equality between two points
     * @returns true when both are equal, false otherwise
     */
    equals(o: any): o is TwistPoint {
        if (!(o instanceof TwistPoint)) {
            return false;
        }

        const a = this.clone();
        a.makeAffine();

        const b = o.clone();
        b.makeAffine();

        return a.x.equals(b.x) && a.y.equals(b.y) && a.z.equals(b.z) && a.t.equals(b.t);
    }

    /**
     * Get the string representation of the point
     * @returns the string representation
     */
    toString(): string {
        const cpy = this.clone();
        cpy.makeAffine();

        return `(${cpy.x.toString()},${cpy.y.toString()},${cpy.z.toString()})`;
    }
}
