import BN from 'bn.js';
import CurvePoint from './curve-point';
import TwistPoint from './twist-point';
import GfP2 from './gfp2';
import GfP12 from './gfp12';
import GfP6 from './gfp6';
import { optimalAte } from './opt-ate';

export type BNType = number | string | number[] | Buffer | BN;

export class G1 {
    private static ELEM_SIZE = 256/8;
    private static MARSHAL_SIZE = G1.ELEM_SIZE * 2;

    private p: CurvePoint;

    constructor(k?: BNType) {
        this.p = new CurvePoint();

        if (k) {
            this.scalarBaseMul(new BN(k));
        }
    }

    getPoint(): CurvePoint {
        return this.p;
    }

    setInfinity(): void {
        this.p.setInfinity();
    }

    isInfinity(): boolean {
        return this.p.isInfinity();
    }

    scalarBaseMul(k: BN): void {
        this.p.mul(CurvePoint.generator, k);
    }

    scalarMul(a: G1, k: BN): void {
        this.p.mul(a.p, k);
    }

    add(a: G1, b: G1): void {
        this.p.add(a.p, b.p);
    }

    neg(a: G1): void {
        this.p.negative(a.p);
    }

    marshal(): Buffer {
        const p = this.p.clone();
        const buf = Buffer.alloc(G1.MARSHAL_SIZE, 0);

        if (p.isInfinity()) {
            return buf;
        }

        p.makeAffine();

        const xBytes = p.getX().toBytes();
        const yBytes = p.getY().toBytes();

        return Buffer.concat([xBytes, yBytes]);
    }

    unmarshal(bytes: Buffer): void {
        if (bytes.length != G1.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for a G1 point');
        }

        this.p = new CurvePoint(bytes.slice(0, G1.ELEM_SIZE), bytes.slice(G1.ELEM_SIZE), 1, 1);

        if (this.p.getX().isZero() && this.p.getY().isZero()) {
            this.p.setInfinity();
            return;
        }

        if (!this.p.isOnCurve()) {
            throw new Error('malformed G1 point');
        }
    }

    equals(o: any): o is G1 {
        if (!(o instanceof G1)) {
            return false;
        }

        return this.p.equals(o.p);
    }

    toString(): string {
        return `bn256.G1${this.p.toString()}`;
    }
}

export class G2 {
    p: TwistPoint;

    private static ELEM_SIZE = 256 / 8;
    private static MARSHAL_SIZE = G2.ELEM_SIZE * 4;

    constructor(k?: BNType) {
        this.p = new TwistPoint();

        if (k) {
            this.scalarBaseMul(new BN(k));
        }
    }

    getPoint(): TwistPoint {
        return this.p;
    }

    setInfinity(): void {
        this.p.setInfinity();
    }

    isInfinity(): boolean {
        return this.p.isInfinity();
    }

    scalarBaseMul(k?: BN): void {
        this.p.mul(TwistPoint.generator, k);
    }

    scalarMul(a: G2, k: BN): void {
        this.p.mul(a.p, k);
    }

    add(a: G2, b: G2): void {
        this.p.add(a.p, b.p);
    }

    neg(a: G2) {
        this.p.neg(a.p);
    }

    marshal(): Buffer {
        if (this.isInfinity()) {
            return Buffer.alloc(G2.MARSHAL_SIZE, 0);
        }

        const t = this.clone();
        t.p.makeAffine();

        const xxBytes = t.p.getX().getX().toBytes();
        const xyBytes = t.p.getX().getY().toBytes();
        const yxBytes = t.p.getY().getX().toBytes();
        const yyBytes = t.p.getY().getY().toBytes();

        return Buffer.concat([xxBytes, xyBytes, yxBytes, yyBytes]);
    }

    unmarshal(bytes: Buffer): void {
        if (bytes.length !== G2.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for G2 point');
        }

        const xxBytes = bytes.slice(0, G2.ELEM_SIZE);
        const xyBytes = bytes.slice(G2.ELEM_SIZE, G2.ELEM_SIZE * 2);
        const yxBytes = bytes.slice(G2.ELEM_SIZE * 2, G2.ELEM_SIZE * 3);
        const yyBytes = bytes.slice(G2.ELEM_SIZE * 3);

        this.p = new TwistPoint(
            new GfP2(xxBytes, xyBytes),
            new GfP2(yxBytes, yyBytes),
            GfP2.one(),
            GfP2.one(),
        );

        if (this.p.getX().isZero() && this.p.getY().isZero()) {
            this.p.setInfinity();
            return;
        }
        
        if (!this.p.isOnCurve()) {
            throw new Error('malformed G2 point');
        }
    }

    clone(): G2 {
        const t = new G2();
        t.p = this.p.clone();

        return t;
    }

    equals(o: any): o is G2 {
        if (!(o instanceof G2)) {
            return false;
        }

        return this.p.equals(o.p);
    }

    toString(): string {
        return `bn256.G2${this.p.toString()}`;
    }
}

export class GT {
    private static ELEM_SIZE = 256 / 8;
    private static MARSHAL_SIZE = GT.ELEM_SIZE * 12;

    public static pair(g1: G1, g2: G2): GT {
        return optimalAte(g1, g2);
    }

    public static one(): GT {
        return new GT(GfP12.one());
    }

    private g: GfP12;

    constructor(g?: GfP12) {
        this.g = g || new GfP12();
    }

    isOne(): boolean {
        return this.g.isOne();
    }

    scalarMul(a: GT, k: BN): void {
        this.g = a.g.exp(k);
    }

    add(a: GT, b: GT): void {
        this.g = a.g.mul(b.g);
    }

    neg(a: GT): void {
        this.g = a.g.invert();
    }

    marshal(): Buffer {
        const xxxBytes = this.g.getX().getX().getX().toBytes();
        const xxyBytes = this.g.getX().getX().getY().toBytes();
        const xyxBytes = this.g.getX().getY().getX().toBytes();
        const xyyBytes = this.g.getX().getY().getY().toBytes();
        const xzxBytes = this.g.getX().getZ().getX().toBytes();
        const xzyBytes = this.g.getX().getZ().getY().toBytes();
        const yxxBytes = this.g.getY().getX().getX().toBytes();
        const yxyBytes = this.g.getY().getX().getY().toBytes();
        const yyxBytes = this.g.getY().getY().getX().toBytes();
        const yyyBytes = this.g.getY().getY().getY().toBytes();
        const yzxBytes = this.g.getY().getZ().getX().toBytes();
        const yzyBytes = this.g.getY().getZ().getY().toBytes();

        return Buffer.concat([
            xxxBytes, xxyBytes, xyxBytes,
            xyyBytes, xzxBytes, xzyBytes,
            yxxBytes, yxyBytes, yyxBytes,
            yyyBytes, yzxBytes, yzyBytes,
        ]);
    }

    unmarshal(bytes: Buffer): void {
        if (bytes.length !== GT.MARSHAL_SIZE) {
            throw new Error('wrong buffer size for a GT point');
        }

        const n = GT.ELEM_SIZE;
        const xxxBytes = bytes.slice(0, n);
        const xxyBytes = bytes.slice(n, n*2);
        const xyxBytes = bytes.slice(n*2, n*3);
        const xyyBytes = bytes.slice(n*3, n*4);
        const xzxBytes = bytes.slice(n*4, n*5);
        const xzyBytes = bytes.slice(n*5, n*6);
        const yxxBytes = bytes.slice(n*6, n*7);
        const yxyBytes = bytes.slice(n*7, n*8);
        const yyxBytes = bytes.slice(n*8, n*9);
        const yyyBytes = bytes.slice(n*9, n*10);
        const yzxBytes = bytes.slice(n*10, n*11);
        const yzyBytes = bytes.slice(n*11);

        this.g = new GfP12(
            new GfP6(new GfP2(xxxBytes, xxyBytes), new GfP2(xyxBytes, xyyBytes), new GfP2(xzxBytes, xzyBytes)),
            new GfP6(new GfP2(yxxBytes, yxyBytes), new GfP2(yyxBytes, yyyBytes), new GfP2(yzxBytes, yzyBytes)),
        );
    }

    equals(o: any): o is GT {
        if (!(o instanceof GT)) {
            return false;
        }

        return this.g.equals(o.g);
    }

    toString(): string {
        return `bn256.GT${this.g.toString()}`;
    }
}
