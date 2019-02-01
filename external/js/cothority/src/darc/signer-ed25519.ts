import { curve, sign, Point, Scalar } from "@dedis/kyber";
import Signer from "./signer";
import IdentityEd25519 from "./identity-ed25519";
import Signature from "./signature";

const ed25519 = curve.newCurve("edwards25519");
const { schnorr } = sign;

export default class SignerEd25519 extends IdentityEd25519 implements Signer {
    private priv: Scalar;

    constructor(pub: Point, priv: Scalar) {
        super({ point: pub.toProto() });
        this.priv = priv;
    }

    get secret(): Scalar {
        return this.priv;
    }

    /** @inheritdoc */
    sign(msg: Buffer): Signature {
        return new Signature({
            signature: schnorr.sign(ed25519, this.priv, msg), 
            signer: this.toWrapper(),
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
