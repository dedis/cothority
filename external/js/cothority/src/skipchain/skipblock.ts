import { Point, sign } from "@dedis/kyber";
import { BN256G1Point, BN256G2Point } from "@dedis/kyber/pairing/point";
import { createHash } from "crypto";
import { Message, Properties } from "protobufjs/light";
import { Roster } from "../network/proto";
import { registerMessage } from "../protobuf";

const EMPTY_BUFFER = Buffer.allocUnsafe(0);

const {bls, Mask} = sign;

/**
 * Convert an integer into a little-endian buffer
 *
 * @param v The number to convert
 * @returns a 32bits buffer
 */
function int2buf(v: number): Buffer {
    const b = Buffer.allocUnsafe(4);
    b.writeInt32LE(v, 0);

    return b;
}

export class SkipBlock extends Message<SkipBlock> {

    /**
     * Getter for the forward links
     *
     * @returns the list of forward links
     */
    get forwardLinks(): ForwardLink[] {
        return this.forward;
    }
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("SkipBlock", SkipBlock, Roster, ForwardLink);
    }

    readonly hash: Buffer;
    readonly index: number;
    readonly height: number;
    readonly maxHeight: number;
    readonly baseHeight: number;
    readonly backlinks: Buffer[];
    readonly verifiers: Buffer[];
    readonly genesis: Buffer;
    readonly data: Buffer;
    readonly roster: Roster;
    readonly forward: ForwardLink[];
    readonly payload: Buffer;

    constructor(props?: Properties<SkipBlock>) {
        super(props);

        this.backlinks = this.backlinks || [];
        this.verifiers = this.verifiers || [];
        this.forward = this.forward || [];
        this.hash = Buffer.from(this.hash || EMPTY_BUFFER);
        this.data = Buffer.from(this.data || EMPTY_BUFFER);
        this.genesis = Buffer.from(this.genesis || EMPTY_BUFFER);
        this.payload = Buffer.from(this.payload || EMPTY_BUFFER);
    }

    /**
     * Calculate the hash of the block
     *
     * @returns the hash
     */
    computeHash(): Buffer {
        const h = createHash("sha256");
        h.update(int2buf(this.index));
        h.update(int2buf(this.height));
        h.update(int2buf(this.maxHeight));
        h.update(int2buf(this.baseHeight));

        for (const bl of this.backlinks) {
            h.update(bl);
        }

        for (const v of this.verifiers) {
            h.update(v);
        }

        h.update(this.genesis);
        h.update(this.data);

        if (this.roster) {
            for (const pub of this.roster.list.map((srvid) => srvid.getPublic())) {
                h.update(pub.marshalBinary());
            }
        }

        return h.digest();
    }
}

export class ForwardLink extends Message<ForwardLink> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("ForwardLink", ForwardLink, Roster, ByzcoinSignature);
    }

    readonly from: Buffer;
    readonly to: Buffer;
    readonly newRoster: Roster;
    readonly signature: ByzcoinSignature;

    constructor(props?: Properties<ForwardLink>) {
        super(props);

        this.from = Buffer.from(this.from || EMPTY_BUFFER);
        this.to = Buffer.from(this.to || EMPTY_BUFFER);
    }

    /**
     * Compute the hash of the forward link
     *
     * @returns the hash
     */
    hash(): Buffer {
        const h = createHash("sha256");
        h.update(this.from);
        h.update(this.to);

        if (this.newRoster) {
            h.update(this.newRoster.id);
        }

        return h.digest();
    }

    /**
     * Verify the signature against the list of public keys
     *
     * @param publics The list of public keys
     * @returns an error if something is wrong, null otherwise
     */
    verify(publics: Point[]): Error {
        if (!this.hash().equals(this.signature.msg)) {
            return new Error("recreated message does not match");
        }

        const agg = this.signature.getAggregate(publics) as BN256G2Point;

        if (!bls.verify(this.signature.msg, agg, this.signature.getSignature())) {
            return new Error("signature not verified");
        }

        return null;
    }
}

export class ByzcoinSignature extends Message<ByzcoinSignature> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("ByzcoinSig", ByzcoinSignature);
    }

    readonly msg: Buffer;
    readonly sig: Buffer;

    constructor(props?: Properties<ByzcoinSignature>) {
        super(props);

        this.msg = Buffer.from(this.msg || EMPTY_BUFFER);
        this.sig = Buffer.from(this.sig || EMPTY_BUFFER);
    }

    /**
     * Get the actual bytes of the signature. The remaining part is the mask.
     *
     * @returns the signature
     */
    getSignature(): Buffer {
        return this.sig.slice(0, new BN256G1Point().marshalSize());
    }

    /**
     * Get the correct aggregation of the public keys using the mask to know
     * which one has been used
     *
     * @param publics The public keys of the roster
     * @returns the aggregated public key for this signature
     */
    getAggregate(publics: Point[]): Point {
        const mask = new Mask(publics, this.sig.slice(new BN256G1Point().marshalSize()));
        return mask.aggregate;
    }
}

SkipBlock.register();
ForwardLink.register();
ByzcoinSignature.register();
