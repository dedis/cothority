import { createHash } from 'crypto';
import { sign, Point } from '@dedis/kyber';
import { Message } from 'protobufjs';
import { Roster } from '../network/proto';
import { registerMessage } from '../protobuf';
import { BN256G2Point, BN256G1Point } from '@dedis/kyber/dist/pairing/point';

const { bls, Mask } = sign;

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

    /**
     * Getter for the forward links
     * 
     * @returns the list of forward links
     */
    get forwardLinks(): ForwardLink[] {
        return this.forward;
    }

    /**
     * Calculate the hash of the block
     * 
     * @returns the hash
     */
    computeHash(): Buffer {
        const h = createHash('sha256');
        /* https://github.com/dedis/cothority/issues/1701
        h.update(int2buf(this.index));
        h.update(int2buf(this.height));
        h.update(int2buf(this.maxHeight));
        h.update(int2buf(this.baseHeight));
        */

        for (const bl of this.backlinks) {
            h.update(bl);
        }

        for (const v of this.verifiers) {
            h.update(v);
        }

        h.update(this.genesis);
        h.update(this.data);

        if (this.roster) {
            for (const pub of this.roster.list.map(srvid => srvid.getPublic())) {
                h.update(pub.marshalBinary());
            }
        }

        return h.digest();
    }
}

export class ForwardLink extends Message<ForwardLink> {
    readonly from: Buffer;
    readonly to: Buffer;
    readonly newRoster: Roster;
    readonly signature: ByzcoinSignature;

    /**
     * Compute the hash of the forward link
     * 
     * @returns the hash
     */
    hash(): Buffer {
        const h = createHash('sha256');
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
            return new Error('recreated message does not match');
        }

        const agg = this.signature.getAggregate(publics) as BN256G2Point;

        if (!bls.verify(this.signature.msg, agg, this.signature.getSignature())) {
            return new Error('signature not verified');
        }

        return null;
    }
}

export class ByzcoinSignature extends Message<ByzcoinSignature> {
    readonly msg: Buffer;
    readonly sig: Buffer;

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

registerMessage('SkipBlock', SkipBlock);
registerMessage('ForwardLink', ForwardLink);
registerMessage('ByzcoinSig', ByzcoinSignature);
