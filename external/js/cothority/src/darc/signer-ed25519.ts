import { curve, sign, Point, Scalar } from "@dedis/kyber";
import Signer from "./signer";
import IdentityEd25519 from "./identity-ed25519";
import Signature from "./signature";
import Identity from "./identity";

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

    /** @inheritdoc */
    get private(): Scalar {
        return this.priv;
    }

    /** @inheritdoc */
    get public(): Point {
        return this.pub;
    }

    /** @inheritdoc */
    get identity(): Identity {
        return new IdentityEd25519({ point: this.pub.marshalBinary() });
    }

    /** @inheritdoc */
    sign(msg: Buffer): Signature {
        return new Signature({
            signature: schnorr.sign(ed25519, this.priv, msg), 
            signer: this.identity.toWrapper(),
        });
    }

    /**
     * Create a Ed25519 signer from the private given as a buffer
     * 
     * @param bytes the private key
     * @returns the new signer
     */
    static fromBytes(bytes: Buffer): SignerEd25519 {
        const priv = ed25519.scalar();
        priv.unmarshalBinary(bytes);
        return new SignerEd25519(ed25519.point().base().mul(priv), priv);
    }
}
