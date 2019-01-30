import { Point, Scalar, curve } from '@dedis/kyber';
import { BLAKE2Xs } from '@stablelib/blake2xs';
import {cloneDeep} from "lodash";

export const Suite = curve.newCurve('edwards25519');

/**
 * Sign a message using (un)linkable ring signature. This method is ported from the Kyber Golang version
 * available at https://github.com/dedis/kyber/blob/master/sign/anon/sig.go. Please refer to the documentation
 * of the given link for detailed instructions. This port stick to the Go implementation, however the hashing function
 * used here is Blake2xs, whereas Blake2xb is used in the Golang version.
 *
 * @param {Buffer} message - the message to be signed
 * @param {Array} anonymitySet - an array containing the public keys of the group
 * @param [linkScope] - ths link scope used for linkable signature
 * @param {Private} privateKey - the private key of the signer
 * @return {RingSig} - the signature
 */
export async function sign(message: Buffer, anonymitySet: Buffer[], linkScope: Buffer, secret: Scalar): Promise<RingSig> {
    let hasLS = (linkScope) && (linkScope !== null);

    let pi = await sortSet(anonymitySet, secret);
    let n = anonymitySet.length;
    let L = anonymitySet.slice(0);

    let linkBase;
    let linkTag: Point;
    if (hasLS) {
        let linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = Suite.point().mul(secret, linkBase);
    }

    let H1pre = signH1pre(linkScope, linkTag, message);

    let u = Suite.scalar().pick();
    let UB = Suite.point().mul(u);
    let UL;
    if (hasLS) {
        UL = Suite.point().mul(u, linkBase);
    }

    let s: Scalar[] = [];
    let c: Scalar[] = [];

    c[(pi + 1) % n] = signH1(H1pre, UB, UL);

    let P = Suite.point();
    let PG = Suite.point();
    let PH: Point;
    if (hasLS) {
        PH = Suite.point();
    }
    for (let i = (pi + 1) % n; i !== pi; i = (i + 1) % n) {
        s[i] = Suite.scalar().pick();
        const pt = Suite.point();
        pt.unmarshalBinary(L[i]);
        PG.add(PG.mul(s[i]), P.mul(c[i], pt));
        if (hasLS) {
            PH.add(PH.mul(s[i], linkBase), P.mul(c[i], linkTag));
        }
        c[(i + 1) % n] = signH1(H1pre, PG, PH);
    }
    s[pi] = Suite.scalar();
    s[pi].mul(secret, c[pi]).sub(u, s[pi]);

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

    let n = anonymitySet.length;
    let L = anonymitySet.slice(0);

    let linkBase, linkTag;
    let sig = decodeSignature(signatureBuffer, !!linkScope);
    if (anonymitySet.length != sig.s.length) {
        return Promise.reject("given anonymity set and signature anonymity set not of equal length")
    }

    if (linkScope) {
        let linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = sig.tag;
    }

    let H1pre = signH1pre(linkScope, linkTag, message);

    let P, PG, PH: Point;
    P = Suite.point();
    PG = Suite.point();
    if (linkScope) {
        PH = Suite.point();
    }
    let s = sig.s;
    let ci = sig.c0;
    for (let i = 0; i < n; i++) {
        PG.add(PG.mul(s[i]), P.mul(ci, L[i]));
        if (linkScope) {
            PH.add(PH.mul(s[i], linkBase), P.mul(ci, linkTag));
        }
        ci = signH1(H1pre, PG, PH);
    }
    if (!ci.equal(sig.c0)) {
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

function createStreamFromBlake(blakeInstance: BLAKE2Xs): (l: number) => Buffer {
    if (!(blakeInstance instanceof BLAKE2Xs)) {
        throw new Error("blakeInstance must be of type Blake2xs");
    }

    function getNextBytes(count: number): Buffer {
        let array = new Uint8Array(count);
        blakeInstance.stream(array);

        return Buffer.from(array);
    }

    return getNextBytes;
}

function signH1pre(linkScope: Buffer, linkTag: Point, message: Buffer): BLAKE2Xs {
    let H1pre = new BLAKE2Xs(undefined, {key: message});

    if (linkScope) {
        H1pre.update(linkScope);
        let tag = linkTag.marshalBinary();
        H1pre.update(tag);
    }

    return H1pre;
}

function signH1(H1pre: BLAKE2Xs, PG: Point, PH: Point): Scalar {
    let H1 = cloneDeep(H1pre);

    let PGb = PG.marshalBinary();
    H1.update(PGb);
    if (PH) {
        let PHb = PH.marshalBinary();
        H1.update(PHb);
    }
    return Suite.scalar().pick(createStreamFromBlake(H1));
}

function decodeSignature(signatureBuffer: Buffer, isLinkableSig: boolean): RingSig {
    let scalarMarshalSize = Suite.scalarLen();
    let pointMarshalSize = Suite.pointLen();

    const c0 = Suite.scalar()
    c0.unmarshalBinary(signatureBuffer.slice(0, pointMarshalSize));

    let S: Scalar[] = [];
    let endIndex = isLinkableSig ? signatureBuffer.length - pointMarshalSize : signatureBuffer.length;
    for (let i = pointMarshalSize; i < endIndex; i += scalarMarshalSize) {
        const t = Suite.scalar();
        t.unmarshalBinary(signatureBuffer.slice(i, i + scalarMarshalSize));
        S.push(t);
    }

    if (isLinkableSig) {
        const tag = Suite.point();
        tag.unmarshalBinary(signatureBuffer.slice(endIndex));
        return new RingSig(c0, S, tag);
    }

    return new RingSig(c0, S);
}

export class RingSig {
    readonly c0: Scalar;
    readonly s: Scalar[];
    readonly tag: Point;

    constructor(c0: Scalar, s: Scalar[], tag?: Point) {
        this.c0 = c0;
        this.s = s;
        this.tag = tag;
    }

    encode(): Buffer {
        let bufs: Buffer[] = [];

        bufs.push(this.c0.marshalBinary());

        for (let scalar of this.s) {
            bufs.push(scalar.marshalBinary());
        }

        if (this.tag) {
            bufs.push(this.tag.marshalBinary());
        }

        return Buffer.concat(bufs);
    }
}

async function sortSet(anonymitySet: Buffer[], privateKey: Scalar): Promise<number>{
    anonymitySet.sort((a, b) => Buffer.compare(a, b));
    let pubKey = Suite.point().base().mul(privateKey).marshalBinary();
    let pi = anonymitySet.findIndex(pub => pub.equals(pubKey));
    if (pi < 0){
        return Promise.reject("didn't find public key in anonymity set")
    }
    return pi;
}