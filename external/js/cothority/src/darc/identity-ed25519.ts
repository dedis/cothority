import { curve, Point, PointFactory, sign } from "@dedis/kyber";
import { Message, Properties } from "protobufjs/light";
import { registerMessage } from "../protobuf";
import { IIdentity } from "./identity-wrapper";

const {schnorr} = sign;
const ed25519 = curve.newCurve("edwards25519");

/**
 * Identity of an Ed25519 signer
 */
export default class IdentityEd25519 extends Message<IdentityEd25519> implements IIdentity {

    /**
     * Get the public key as a point
     */
    get public(): Point {
        if (!this._public) {
            this._public = PointFactory.fromProto(this.point);
        }

        return this._public;
    }
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("IdentityEd25519", IdentityEd25519);
    }

    /**
     * Initialize an IdentityEd25519 from a point.
     */
    static fromPoint(p: Point): IdentityEd25519 {
        return new IdentityEd25519({point: p.toProto()});
    }

    // Protobuf-encoded point
    readonly point: Buffer;

    private _public: Point;

    constructor(props?: Properties<IdentityEd25519>) {
        super(props);
    }

    /** @inheritdoc */
    verify(msg: Buffer, signature: Buffer): boolean {
        return schnorr.verify(ed25519, this.public, msg, signature);
    }

    /** @inheritdoc */
    toBytes(): Buffer {
        return this.point;
    }

    /** @inheritdoc */
    toString() {
        return `ed25519:${this.public.toString().toLowerCase()}`;
    }
}
