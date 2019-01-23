import {Signer} from "~/lib/cothority/darc/Signer";
import {IdentityEd25519} from "~/lib/cothority/darc/IdentityEd25519";
import {Signature} from "~/lib/cothority/darc/Signature";
import {Identity} from "~/lib/cothority/darc/Identity";
import {Log} from "~/lib/Log";

const curve = require("@dedis/kyber-js").curve.newCurve("edwards25519");
const Schnorr = require("@dedis/kyber-js").sign.schnorr;

/**
 * @extends Signer
 */
export class SignerEd25519 extends Signer {
    _pub: any;
    _priv: any;

    constructor(pub, priv) {
        super();
        this._pub = pub;
        this._priv = priv;
    }

    static fromByteArray(bytes): SignerEd25519 {
        const priv = curve.scalar();
        priv.unmarshalBinary(bytes);
        return new SignerEd25519(curve.point().base().mul(priv), priv);
    }

    get private(): any {
        return this._priv;
    }

    get public(): any {
        return this._pub;
    }

    get identity(): Identity {
        return new IdentityEd25519(this._pub);
    }

    sign(msg): Signature {
        return new Signature(Schnorr.sign(curve, this._priv, new Uint8Array(msg)), this.identity);
    }
}
