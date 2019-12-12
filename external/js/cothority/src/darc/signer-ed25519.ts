import { curve, Point, Scalar, sign } from "@dedis/kyber";
import Ed25519Scalar from "@dedis/kyber/curve/edwards25519/scalar";
import { randomBytes } from "crypto-browserify";
import Log from "../log";
import IdentityEd25519 from "./identity-ed25519";
import ISigner from "./signer";

const ed25519 = curve.newCurve("edwards25519");
const {schnorr} = sign;

export default class SignerEd25519 extends IdentityEd25519 implements ISigner {

    get secret(): Scalar {
        return this.priv;
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

    static random(): SignerEd25519 {
        const priv = ed25519.scalar().setBytes(randomBytes(32));
        const pub = ed25519.point().mul(priv);
        return new SignerEd25519(pub, priv);
    }

    private priv: Scalar;

    constructor(pub: Point, priv: Scalar) {
        super({point: pub.toProto()});
        this.priv = priv;
    }

    /** @inheritdoc */
    sign(msg: Buffer): Buffer {
        return schnorr.sign(ed25519, this.priv, msg);
    }
}
