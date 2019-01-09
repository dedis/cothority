import BN = require("bn.js");
import { randomBytes } from "crypto";
import Ed25519 from "./curve";
import { int } from "../../random"
import { Scalar } from "../../index";

export default class Ed25519Scalar implements Scalar {
    ref: {
        arr: any,
        curve: Ed25519
        red: any
    }

    constructor(curve: Ed25519, red: any) {
        this.ref = {
            arr: new BN(0, 16).toRed(red),
            curve: curve,
            red: red,
        };
    }

    marshalSize(): number {
        return 32;
    }

    marshalBinary(): Buffer {
        return Buffer.from(this.ref.arr.fromRed().toArray("le", 32));
    }

    unmarshalBinary(bytes: Buffer): void {
        if (bytes.length > this.marshalSize()) {
            throw new Error("bytes.length > marshalSize");
        }
        this.ref.arr = new BN(bytes, 16, "le").toRed(this.ref.red);
    }

    equal(s2: Ed25519Scalar): boolean {
        return this.ref.arr.fromRed().cmp(s2.ref.arr.fromRed()) == 0;
    }

    set(a: Ed25519Scalar): Ed25519Scalar {
        this.ref = a.ref;
        return this;
    }

    clone(): Scalar {
        return new Ed25519Scalar(this.ref.curve, this.ref.red).setBytes(
            Buffer.from(this.ref.arr.fromRed().toArray("le"))
        );
    }

    zero(): Scalar {
        this.ref.arr = new BN(0, 16).toRed(this.ref.red);
        return this;
    }
    add(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = a.ref.arr.redAdd(b.ref.arr);
        return this;
    }

    sub(a: Ed25519Scalar, b: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = a.ref.arr.redSub(b.ref.arr);
        return this;
    }

    neg(a: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = a.ref.arr.redNeg();
        return this;
    }

    mul(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr);
        return this;
    }

    div(s1: Ed25519Scalar, s2: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = s1.ref.arr.redMul(s2.ref.arr.redInvm());
        return this;
    }

    inv(a: Ed25519Scalar): Ed25519Scalar {
        this.ref.arr = a.ref.arr.redInvm();
        return this;
    }

    one(): Ed25519Scalar {
        this.ref.arr = new BN(1, 16).toRed(this.ref.red);
        return this;
    }
    pick(callback?: (length: number) => Buffer): Scalar {
        callback = callback || randomBytes;
        const bytes = int(this.ref.curve.curve.n, callback);
        this.ref.arr = new BN(bytes, 16).toRed(this.ref.red);
        return this;
    }

    setBytes(bytes: Buffer): Scalar {
        this.ref.arr = new BN(bytes , 16, "le").toRed(this.ref.red);
        return this;
    }

    toString(): string {
        const bytes = this.ref.arr.fromRed().toArray("le", 32);
        return bytes.map(b => ("0" + (b & 0xff).toString(16)).slice(-2)).join("");
    }
}