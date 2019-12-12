import { Point, sign } from "@dedis/kyber";
import { BN256G1Point, BN256G2Point } from "@dedis/kyber/pairing/point";
import { createHash } from "crypto-browserify";
import { Message, Properties } from "protobufjs/light";
import { Roster } from "../network/proto";
import { registerMessage } from "../protobuf";

const EMPTY_BUFFER = Buffer.allocUnsafe(0);

export const BLS_INDEX = 0;
export const BDN_INDEX = 1;

const { bls, bdn, Mask } = sign;

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
    readonly signatureScheme: number;

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

        if (this.signatureScheme > 0) {
            h.update(int2buf(this.signatureScheme));
        }

        return h.digest();
    }
}

/**
 * Compute the minimum number of signatures an aggregate must have
 * according to the total number of nodes
 *
 * @param n The total numner of conodes
 * @returns the minimum number of signatures required
 */
function defaultThreshold(n: number): number {
    // n = 3f + 1 with f the number of faulty nodes
    return n - ((n - 1) / 3);
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
        return this.verifyWithScheme(publics, BLS_INDEX);
    }

    /**
     * Verify the signature against the list of public keys
     * using the specified signature scheme
     *
     * @param publics   The list of public keys
     * @param scheme    The index of the signature scheme
     * @returns an error if something is wrong, null otherwise
     */
    verifyWithScheme(publics: Point[], scheme: number): Error {
        if (!this.hash().equals(this.signature.msg)) {
            return new Error("recreated message does not match");
        }

        const mask = new Mask(publics, this.signature.getMask());
        // Note: we only check that there are enough signatures because if the mask
        // is forged to have only one key for instance, the creation of the mask
        // will fail with a mismatch length
        if (mask.getCountEnabled() < defaultThreshold(mask.getCountTotal())) {
            return new Error("not enough signers");
        }

        switch (scheme) {
            case BLS_INDEX:
                return this.verifyBLS(mask);
            case BDN_INDEX:
                return this.verifyBDN(mask);
            default:
                return new Error("unknown signature scheme");
        }
    }

    private verifyBLS(mask: sign.Mask): Error {
        const agg = mask.aggregate as BN256G2Point;

        if (!bls.verify(this.signature.msg, agg, this.signature.getSignature())) {
            return new Error("BLS signature not verified");
        }

        return null;
    }

    private verifyBDN(mask: sign.Mask): Error {
        if (!bdn.verify(this.signature.msg, mask, this.signature.getSignature())) {
            return new Error("BDN signature not verified");
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
        const mask = new Mask(publics, this.getMask());
        return mask.aggregate;
    }

    /**
     * Get the buffer slice that represents the mask
     *
     * @returns the mask as a buffer
     */
    getMask(): Buffer {
        return this.sig.slice(new BN256G1Point().marshalSize());
    }
}

SkipBlock.register();
ForwardLink.register();
ByzcoinSignature.register();
