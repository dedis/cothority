import BN from "bn.js";
import { eddsa } from "elliptic";
import { randomBytes } from "crypto";
import { Point } from "../../index";
import { BNType } from '../../constants';
import Ed25519Scalar from "./scalar";

const ec = new eddsa("ed25519");

export default class Ed25519Point implements Point {
    public static MARSHAL_ID = Buffer.from('ed.point');

    // TODO: this should be private
    ref: {
        point: any,
    }

    constructor(X?: BNType, Y?: BNType, Z?: BNType, T?: BNType) {
        if (X instanceof Buffer) {
            X = new BN(X, 16, "le");
        }
        if (Y instanceof Buffer) {
            Y = new BN(Y, 16, "le");
        }
        if (Z instanceof Buffer) {
            Z = new BN(Z, 16, "le");
        }
        if (T instanceof Buffer) {
            T = new BN(T, 16, "le");
        }
        // the point reference is stored in a Point reference to make set()
        // consistent.
        this.ref = {
            point: ec.curve.point(X, Y, Z, T),
        };
    }

    /** @inheritdoc */
    null(): Ed25519Point {
        this.ref.point = ec.curve.point(0, 1, 1, 0);
        return this;
    }

    /** @inheritdoc */
    base(): Ed25519Point {
        this.ref.point = ec.curve.point(
            ec.curve.g.getX(),
            ec.curve.g.getY(),
        );
        return this;
    }

    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): Ed25519Point {
        return this.embed(Buffer.from([]), callback);
    }

    /** @inheritdoc */
    set(p: Ed25519Point): Ed25519Point {
        this.ref = p.ref;
        return this;
    }

    /** @inheritdoc */
    clone(): Ed25519Point {
        const { point } = this.ref;
        return new Ed25519Point(point.x, point.y, point.z, point.t);
    }

    /** @inheritdoc */
    embedLen(): number {
        // Reserve the most-significant 8 bits for pseudo-randomness.
        // Reserve the least-significant 8 bits for embedded data length.
        // (Hopefully it's unlikely we'll need >=2048-bit curves soon.)
        return Math.floor((255 - 8 - 8) / 8);
    }

    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): Ed25519Point {
        let dl = this.embedLen();
        if (data.length > dl) {
            throw new Error("data.length > embedLen");
        }

        if (dl > data.length) {
            dl = data.length;
        }

        callback = callback || randomBytes;

        let point_obj = new Ed25519Point();
        while (true) {
            const buff = callback(32);

            if (dl > 0) {
                buff[0] = dl; // encode length in lower 8 bits
                data.copy(buff, 1) // copy data into buff starting from the 2nd position
            }

            try {
                point_obj.unmarshalBinary(buff);
            } catch (e) {
                continue; // try again
            }
            if (dl == 0) {
                point_obj.ref.point = point_obj.ref.point.mul(new BN(8));
                if (point_obj.ref.point.isInfinity()) {
                    continue; // unlucky
                }
                return point_obj;
            }

            let q = point_obj.clone();
            q.ref.point = q.ref.point.mul(ec.curve.n);
            if (q.ref.point.isInfinity()) {
                return point_obj;
            }
        }
    }

    /** @inheritdoc */
    data(): Buffer {
        const bytes = this.marshalBinary();
        const dl = bytes[0];
        if (dl > this.embedLen()) {
            throw new Error("invalid embedded data length");
        }
        return bytes.slice(1, dl + 1);
    }

    /** @inheritdoc */
    add(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point {
        const point = p1.ref.point;
        this.ref.point = ec.curve
            .point(point.x, point.y, point.z, point.t)
            .add(p2.ref.point);
        return this;
    }

    /** @inheritdoc */
    sub(p1: Ed25519Point, p2: Ed25519Point): Ed25519Point {
        const point = p1.ref.point;
        this.ref.point = ec.curve
            .point(point.x, point.y, point.z, point.t)
            .add(p2.ref.point.neg());
        return this;
    }

    /** @inheritdoc */
    neg(p: Ed25519Point): Ed25519Point {
        this.ref.point = p.ref.point.neg();
        return this;
    }

    /** @inheritdoc */
    mul(s: Ed25519Scalar, p?: Ed25519Point): Ed25519Point {
        p = p || null;
        const arr = s.ref.arr;
        this.ref.point =
        p !== null ? p.ref.point.mul(arr) : ec.curve.g.mul(arr);
        return this;
    }

    /** @inheritdoc */
    marshalSize(): number {
        return 32;
    }

    /** @inheritdoc */
    marshalBinary(): Buffer {
        this.ref.point.normalize();

        const buffer = this.ref.point.getY().toArray("le", 32);
        buffer[31] ^= (this.ref.point.x.isOdd() ? 1 : 0) << 7;

        return Buffer.from(buffer);
    }

    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void {
        // we create a copy because the array might be modified
        const buff = Buffer.from(bytes);

        const odd = buff[31] >> 7 === 1;

        buff[31] &= 0x7f;
        let bnp = new BN(buff, 16, "le");
        if (bnp.cmp(ec.curve.p) >= 0) {
            throw new Error("bytes > p");
        }
        this.ref.point = ec.curve.pointFromY(bnp, odd);
    }

    inspect(): string {
        return this.toString()
    }

    /** @inheritdoc */
    equals(p2: Ed25519Point): boolean {
        const b1 = this.marshalBinary();
        const b2 = p2.marshalBinary();
        for (var i = 0; i < 32; i++) {
            if (b1[i] !== b2[i]) {
                return false;
            }
        }
        return true;
    }

    /** @inheritdoc */
    toString(): string {
        const bytes = this.marshalBinary();
        return Array.from(bytes, b => ("0" + (b & 0xff).toString(16)).slice(-2)).join("");
    }

    /** @inheritdoc */
    toProto(): Buffer {
        return Buffer.concat([Ed25519Point.MARSHAL_ID, this.marshalBinary()]);
    }
}