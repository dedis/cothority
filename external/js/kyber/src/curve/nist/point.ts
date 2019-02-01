import BN from 'bn.js';
import { randomBytes } from "crypto";
import { zeroBN, BNType } from "../../constants";
import { Point } from "../../index";
import Weierstrass from "./curve";
import NistScalar from "./scalar";

/**
* Represents a Point on the nist curve
*
* The value of the parameters is expected in little endian form if being
* passed as a buffer
*/
export default class NistPoint implements Point {
    ref: { curve: Weierstrass, point: any }
    constructor(curve: Weierstrass, x?: BNType, y?: BNType) {
        if (x instanceof Buffer) {
            x = new BN(x, 16, "le");
        }
        if (y instanceof Buffer) {
            y = new BN(y, 16, "le");
        }
        
        // the point reference is stored in an object to make set()
        // consistent.
        this.ref = {
            curve: curve,
            point: curve.curve.point(x, y)
        };
    }
    
    /** @inheritdoc */
    set(p2: NistPoint): NistPoint {
        this.ref = p2.ref;
        return this;
    }
    
    /** @inheritdoc */
    clone(): NistPoint{
        const point = this.ref.point;
        return new NistPoint(this.ref.curve, point.x, point.y);
    }
    
    /** @inheritdoc */
    null(): NistPoint {
        this.ref.point = this.ref.curve.curve.point(null, null);
        return this;
    }
    
    /** @inheritdoc */
    base(): NistPoint {
        const g = this.ref.curve.curve.g;
        this.ref.point = this.ref.curve.curve.point(g.x, g.y);
        return this;
    }
    
    /** @inheritdoc */
    embedLen(): number {
        // Reserve the most-significant 8 bits for pseudo-randomness.
        // Reserve the least-significant 8 bits for embedded data length.
        // (Hopefully it's unlikely we'll need >=2048-bit curves soon.)
        return (this.ref.curve.curve.p.bitLength() - 8 - 8) >> 3;
    }
    
    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): NistPoint {
        let l = this.ref.curve.coordLen();
        let dl = this.embedLen();
        if (data.length > dl) {
            throw new Error("data.length > dl");
        }
        
        if (dl > data.length) {
            dl = data.length;
        }
        
        callback = callback || randomBytes;
        
        while (true) {
            const bitLen = this.ref.curve.curve.p.bitLength();
            const buffLen = bitLen >> 3;
            let buff = callback(buffLen);
            let highbits = bitLen & 7;
            if (highbits != 0) {
                buff[0] &= ~(0xff << highbits);
            }
            
            if (dl > 0) {
                buff[l - 1] = dl; // encode length in lower 8 bits
                data.copy(buff, l - dl -1);
            }
            //console.log(bytes);
            
            let x = new BN(buff, 16, "be");
            if (x.cmp(this.ref.curve.curve.p) > 0) {
                continue;
            }
            
            let xRed = x.toRed(this.ref.curve.curve.red);
            let aX = xRed.redMul(new BN(this.ref.curve.curve.a));
            // y^2 = x^3 + ax + b
            let y2 = xRed.redSqr()
                .redMul(xRed)
                .redAdd(aX)
                .redAdd(new BN(this.ref.curve.curve.b));
            
            let y = y2.redSqrt();
            
            let b = callback(1);
            if ((b[0] & 0x80) !== 0) {
                y = this.ref.curve.curve.p.sub(y).toRed(this.ref.curve.curve.red);
            }
            
            // check if it is a valid point
            let y2t = y.redSqr();
            if (y2t.cmp(y2) === 0) {
                return new NistPoint(this.ref.curve, xRed, y);
            }
        }
    }
    
    /** @inheritdoc */
    data(): Buffer {
        const l = this.ref.curve.coordLen();
        let b = Buffer.from(this.ref.point.x.fromRed().toArray("be", l));
        const dl = b[l - 1];
        if (dl > this.embedLen()) {
            throw new Error("invalid embed data length");
        }
        return b.slice(l - dl - 1, l - 1);
    }
    
    /** @inheritdoc */
    add(p1: NistPoint, p2: NistPoint): NistPoint {
        const point = p1.ref.point;
        this.ref.point = this.ref.curve.curve
            .point(point.x, point.y)
            .add(p2.ref.point);
        return this;
    }
    
    /** @inheritdoc */
    sub(p1: NistPoint, p2: NistPoint): NistPoint {
        const point = p1.ref.point;
        this.ref.point = this.ref.curve.curve
        .point(point.x, point.y)
        .add(p2.ref.point.neg());
        return this;
    }
    
    /** @inheritdoc */
    neg(p: NistPoint): NistPoint {
        this.ref.point = p.ref.point.neg();
        return this;
    }
    
    /** @inheritdoc */
    mul(s: NistScalar, p?: NistPoint): NistPoint {
        p = p || null;
        const arr = s.ref.arr.fromRed();
        this.ref.point =
        p !== null ? p.ref.point.mul(arr) : this.ref.curve.curve.g.mul(arr);
        return this;
    }
    
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): NistPoint {
        callback = callback || null
        return this.embed(Buffer.from([]), callback);
    }
    
    /** @inheritdoc */
    marshalSize(): number {
        // uncompressed ANSI X9.62 representation
        return this.ref.curve.pointLen();
    }
    
    /** @inheritdoc */
    marshalBinary(): Buffer {
        const byteLen = this.ref.curve.coordLen();
        const buf = Buffer.allocUnsafe(this.ref.curve.pointLen());
        buf[0] = 4; // uncompressed point
        
        let xBytes = Buffer.from(this.ref.point.x.fromRed().toArray("be"));
        xBytes.copy(buf, 1 + byteLen - xBytes.length);
        let yBytes = Buffer.from(this.ref.point.y.fromRed().toArray("be"));
        yBytes.copy(buf, 1 + 2 * byteLen - yBytes.length);
        
        return buf;
    }
    
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void {
        const byteLen = this.ref.curve.coordLen();
        if (bytes.length != 1 + 2 * byteLen) {
            throw new Error();
        }
        // not an uncompressed point
        if (bytes[0] != 4) {
            throw new Error("unmarshalBinary only accepts uncompressed point");
        }
        let x = new BN(bytes.slice(1, 1 + byteLen), 16);
        let y = new BN(bytes.slice(1 + byteLen), 16);
        if (x.cmp(zeroBN) === 0 && y.cmp(zeroBN) === 0) {
            this.ref.point = this.ref.curve.curve.point(null, null);
            return;
        }
        this.ref.point = this.ref.curve.curve.point(x, y);
        if (!this.ref.curve.curve.validate(this.ref.point)) {
            throw new Error("point is not on curve");
        }
    }

    /** @inheritdoc */
    equals(p2: NistPoint): boolean {
        if (this.ref.point.isInfinity() ^ p2.ref.point.isInfinity()) {
            return false;
        }
        if (this.ref.point.isInfinity() & p2.ref.point.isInfinity()) {
            return true;
        }
        return (
            this.ref.point.x.cmp(p2.ref.point.x) === 0 &&
            this.ref.point.y.cmp(p2.ref.point.y) === 0
        );
    }

    inspect(): string {
        return this.toString()
    }
    
    /** @inheritdoc */
    toString(): string {
        if (this.ref.point.inf) {
            return "(0,0)";
        }
        return (
            "(" +
            this.ref.point.x.fromRed().toString(10) +
            "," +
            this.ref.point.y.fromRed().toString(10) +
            ")"
        );
    }

    /** @inheritdoc */
    toProto(): Buffer {
        throw new Error('not implemented');
    }
}