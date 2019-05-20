"use strict";

import { curve, Point, Scalar } from "@dedis/kyber";
import { BLAKE2Xs } from "@stablelib/blake2xs";
import { cloneDeep } from "lodash";

export const ed25519 = new curve.edwards25519.Curve();

/**
 * Convenience class to wrap a linkable ring signature.
 */
export class RingSig {
    constructor(public C0: Scalar, public S: Scalar[], public tag: Point = null) {
    }

    encode(): Buffer {
        const array: Buffer[] = [];

        array.push(this.C0.marshalBinary());

        for (const scalar of this.S) {
            array.push(scalar.marshalBinary());
        }

        if (this.tag) {
            array.push(this.tag.marshalBinary());
        }

        return Buffer.concat(array);
    }
}

/**
 * Sign a message using (un)linkable ring signature. This method is ported from the Kyber Golang version
 * available at https://github.com/dedis/kyber/blob/master/sign/anon/sig.go. Please refer to the documentation
 * of the given link for detailed instructions. This port stick to the Go implementation, however the hashing function
 * used here is Blake2xs, whereas Blake2xb is used in the Golang version.
 *
 * @param {Buffer} message - the message to be signed
 * @param {Array} anonymitySet - an array containing the public keys of the group
 * @param [linkScope] - ths link scope used for linkable signature
 * @param {Scalar} privateKey - the private key of the signer
 * @return {RingSig} - the signature
 */
export async function Sign(message: Buffer, anonymitySet: Point[],
                           linkScope: Buffer, privateKey: Scalar):
    Promise<RingSig> {
    const hasLS = (linkScope) && (linkScope !== null);

    const pi = await sortSet(anonymitySet, privateKey);
    const n = anonymitySet.length;
    const L = anonymitySet.slice(0);

    let linkBase;
    let linkTag: Point;
    if (hasLS) {
        const linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = ed25519.point().pick(createStreamFromBlake(linkStream));
        linkTag = ed25519.point().mul(privateKey, linkBase);
    }

    // tslint:disable-next-line
    const H1pre = signH1pre(linkScope, linkTag, message);

    const u = ed25519.scalar().pick();
    const UB = ed25519.point().mul(u);
    let UL;
    if (hasLS) {
        UL = ed25519.point().mul(u, linkBase);
    }

    const s: any[] = [];
    const c: Scalar[] = [];

    c[(pi + 1) % n] = signH1(H1pre, UB, UL);

    const P = ed25519.point();
    const PG = ed25519.point();
    let PH: Point;
    if (hasLS) {
        PH = ed25519.point();
    }
    for (let i = (pi + 1) % n; i !== pi; i = (i + 1) % n) {
        s[i] = ed25519.scalar().pick();
        PG.add(PG.mul(s[i]), P.mul(c[i], L[i]));
        if (hasLS) {
            PH.add(PH.mul(s[i], linkBase), P.mul(c[i], linkTag));
        }
        c[(i + 1) % n] = signH1(H1pre, PG, PH);
    }
    s[pi] = ed25519.scalar();
    s[pi].mul(privateKey, c[pi]).sub(u, s[pi]);

    return new RingSig(c[0], s, linkTag);
}

/**
 * Verify the signature of a message  a message using (un)linkable ring signature. This method is ported from
 * the Kyber Golang version available at https://github.com/dedis/kyber/blob/master/sign/anon/sig.go. Please refer
 * to the documentation of the given link for detailed instructions. This port stick to the Go implementation, however
 * the hashing function used here is Blake2xs, whereas Blake2xb is used in the Golang version.
 *
 * @param {Kyber.Curve} suite - the crypto suite used for the sign process
 * @param {Uint8Array} message - the message to be signed
 * @param {Array} anonymitySet - an array containing the public keys of the group
 * @param [linkScope] - ths link scope used for linkable signature
 * @param signatureBuffer - the signature the will be verified
 * @return {SignatureVerification} - contains the property of the verification
 */
export async function Verify(message: Buffer, anonymitySet: Point[], linkScope: Buffer, signatureBuffer: Buffer):
    Promise<SignatureVerification> {
    if (!(signatureBuffer instanceof Uint8Array)) {
        return Promise.reject("signatureBuffer must be Uint8Array");
    }
    anonymitySet.sort((a, b) => {
        return Buffer.compare(a.marshalBinary(), b.marshalBinary());
    });

    const n = anonymitySet.length;
    const L = anonymitySet.slice(0);

    let linkBase: Point;
    let linkTag: Point;
    const sig = decodeSignature(signatureBuffer, !!linkScope);
    if (anonymitySet.length !== sig.S.length) {
        return Promise.reject("given anonymity set and signature anonymity set not of equal length");
    }

    if (linkScope) {
        const linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = ed25519.point().pick(createStreamFromBlake(linkStream));
        linkTag = sig.tag;
    }

    // tslint:disable-next-line
    const H1pre = signH1pre(linkScope, linkTag, message);

    const P = ed25519.point();
    const PG = ed25519.point();
    let PH: Point;
    if (linkScope) {
        PH = ed25519.point();
    }
    const s = sig.S;
    let ci = sig.C0;
    for (let i = 0; i < n; i++) {
        PG.add(PG.mul(s[i]), P.mul(ci, L[i]));
        if (linkScope) {
            PH.add(PH.mul(s[i], linkBase), P.mul(ci, linkTag));
        }
        ci = signH1(H1pre, PG, PH);
    }
    if (!ci.equals(sig.C0)) {
        return new SignatureVerification(false);
    }

    if (linkScope) {
        return new SignatureVerification(true, linkTag);
    }

    return new SignatureVerification(true);
}

export class SignatureVerification {
    constructor(public valid: boolean, public tag: Point = null) {
    }
}

function createStreamFromBlake(blakeInstance: BLAKE2Xs): (a: number) => Buffer {
    if (!(blakeInstance instanceof BLAKE2Xs)) {
        throw new Error("blakeInstance must be of type Blake2xs");
    }

    function getNextBytes(count: number): Buffer {
        if (!Number.isInteger(count)) {
            throw new Error("count must be a integer");
        }
        const array = new Uint8Array(count);
        blakeInstance.stream(array);
        return Buffer.from(array);
    }

    return getNextBytes;
}

function signH1pre(linkScope: Buffer, linkTag: Point, message: Buffer): any {
    // tslint:disable-next-line
    const H1pre = new BLAKE2Xs(undefined, {key: message});

    if (linkScope) {
        H1pre.update(linkScope);
        const tag = linkTag.marshalBinary();
        H1pre.update(tag);
    }

    return H1pre;
}

// tslint:disable-next-line
function signH1(H1pre: BLAKE2Xs, PG: Point, PH: Point): Scalar {
    const H1 = cloneDeep(H1pre);

    // tslint:disable-next-line
    const PGb = PG.marshalBinary();
    H1.update(PGb);
    if (PH) {
        // tslint:disable-next-line
        const PHb = PH.marshalBinary();
        H1.update(PHb);
    }
    return ed25519.scalar().pick(createStreamFromBlake(H1));
}

function decodeSignature(signatureBuffer: Buffer, isLinkableSig: boolean): RingSig {
    // tslint:disable-next-line
    const scalarMarshalSize = ed25519.scalar().marshalSize();
    const pointMarshalSize = ed25519.point().marshalSize();
    const c0 = ed25519.scalar();
    c0.unmarshalBinary(signatureBuffer.slice(0, pointMarshalSize));

    const S: Scalar[] = [];
    const endIndex = isLinkableSig ? signatureBuffer.length - pointMarshalSize : signatureBuffer.length;
    for (let i = pointMarshalSize; i < endIndex; i += scalarMarshalSize) {
        const pr = ed25519.scalar();
        pr.unmarshalBinary(signatureBuffer.slice(i, i + scalarMarshalSize));
        S.push(pr);
    }

    const fields = new RingSig(c0, S);

    if (isLinkableSig) {
        fields.tag = ed25519.point();
        fields.tag.unmarshalBinary(signatureBuffer.slice(endIndex));
    }

    return fields;
}

async function sortSet(anonymitySet: Point[], privateKey: Scalar): Promise<number> {
    anonymitySet.sort((a, b) => {
        return Buffer.compare(a.marshalBinary(), b.marshalBinary());
    });
    const pubKey = ed25519.point().base().mul(privateKey);
    const pi = anonymitySet.findIndex((pub) => pub.equals(pubKey));
    if (pi < 0) {
        return Promise.reject("didn't find public key in anonymity set");
    }
    return pi;
}
