import BN from "bn.js";
import { randomBytes } from "crypto"
import { int } from "../../random"
import { Scalar } from "../../index"
import Weierstrass from "./curve";

export default class NistScalar implements Scalar {
    ref: { arr: any, red: any, curve: Weierstrass}
    constructor(curve: Weierstrass, red: any) {
        this.ref = {
            arr: new BN(0, 16).toRed(red),
            red: red,
            curve: curve
        };
    }
    
    /** @inheritdoc */
    set(a: NistScalar): NistScalar {
        this.ref = a.ref;
        return this;
    }
    
    /** @inheritdoc */
    clone(): NistScalar {
        return new NistScalar(this.ref.curve, this.ref.red).setBytes(
            Buffer.from(this.ref.arr.fromRed().toArray("be"))
        );
    }
    
    /** @inheritdoc */
    zero(): NistScalar {
        this.ref.arr = new BN(0, 16).toRed(this.ref.red);
        return this;
    }
    
    /** @inheritdoc */
    add(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redAdd(s2.ref.arr);
        return this;
    }
    
    /** @inheritdoc */
    sub(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redSub(s2.ref.arr);
        return this;
    }
    
    /** @inheritdoc */
    neg(a: NistScalar): NistScalar {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }
    
    /** @inheritdoc */
    one(): NistScalar {
        this.ref.arr = new BN(1, 16).toRed(this.ref.red);
        return this;
    }
    
    /** @inheritdoc */
    mul(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
        return this;
    }
    
    /** @inheritdoc */
    div(s1: NistScalar, s2: NistScalar): NistScalar {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }
    
    /** @inheritdoc */
    inv(a: NistScalar): NistScalar {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }
    
    /** @inheritdoc */
    setBytes(b: Buffer): NistScalar{
        this.ref.arr = new BN(b, 16, "be").toRed(this.ref.red);
        return this;
    }
    
    /** @inheritdoc */
    bytes(): Buffer{
        return Buffer.from(this.ref.arr.fromRed().toArray("be"));
    }
    
    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): NistScalar {
        callback = callback || randomBytes;
        let bytes = int(this.ref.curve.curve.n, callback);
        this.setBytes(bytes);
        return this;
    }
    
    /** @inheritdoc */
    marshalSize(): number {
        return this.ref.curve.scalarLen();
    }
    
    /** @inheritdoc */
    marshalBinary(): Buffer {
        return Buffer.from(
            this.ref.arr.fromRed().toArray("be", this.ref.curve.scalarLen())
        );
    }
    
    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void {
        if (bytes.length != this.marshalSize()) {
            throw new Error("bytes.length > marshalSize");
        }

        const bnObj = new BN(bytes, 16);
        if (bnObj.cmp(this.ref.curve.curve.n) > 0) {
            throw new Error("bytes > q");
        }
        this.setBytes(bytes);
    }

    /** @inheritdoc */
    equals(s2: NistScalar): boolean {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }

    /** @inheritdoc */
    toString(): string {
        let bytes = Buffer.from(this.ref.arr.fromRed().toArray("be"));
        return Array.from(bytes, b => {
            return ("0" + (b & 0xff).toString(16)).slice(-2);
        }).join("");
    }

    inspect(): string {
        return this.toString()
    }
}