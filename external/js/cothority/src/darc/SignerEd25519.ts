import { Signer } from "./Signer";
import { IdentityEd25519 } from "./IdentityEd25519";
import { Signature } from "./Signature";
import { Identity } from "./Identity";
import { curve, sign, Point, Scalar } from "@dedis/kyber";

const ed25519 = curve.newCurve("edwards25519");
const { schnorr } = sign;

export default class SignerEd25519 extends Signer {
    private pub: Point;
    private priv: Scalar;

    constructor(pub: Point, priv: Scalar) {
        super();
        this.pub = pub;
        this.priv = priv;
    }

    static fromBytes(bytes: Buffer): SignerEd25519 {
        const priv = ed25519.scalar();
        priv.unmarshalBinary(bytes);
        return new SignerEd25519(ed25519.point().base().mul(priv), priv);
    }

    get private(): Scalar {
        return this.priv;
    }

    get public(): Point {
        return this.pub;
    }

    get identity(): Identity {
        return new IdentityEd25519({ point: this.pub.marshalBinary() });
    }

    sign(msg: Buffer): Signature {
        return new Signature({
            signature: schnorr.sign(ed25519, this.priv, msg), 
            signer: this.identity.toWrapper(),
        });
    }
}
