import { BLAKE2Xs } from '@stablelib/blake2xs';
import { curve, Scalar, Point } from "../..";
import { cloneDeep } from 'lodash';

export const Suite = curve.newCurve('edwards25519');

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

    public static fromBytes(signatureBuffer: Buffer, isLinkableSig: boolean): RingSig {
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
}

export function sign(message: Buffer, anonymitySet: Point[], secret: Scalar, linkScope?: Buffer) {
    let hasLS = linkScope && (linkScope !== null);

    let pi = findSecretIndex(anonymitySet, secret);
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
        PG.add(PG.mul(s[i]), P.mul(c[i], L[i]));
        if (hasLS) {
            PH.add(PH.mul(s[i], linkBase), P.mul(c[i], linkTag));
        }
        c[(i + 1) % n] = signH1(H1pre, PG, PH);
    }
    s[pi] = Suite.scalar();
    s[pi].mul(secret, c[pi]).sub(u, s[pi]);

    return new RingSig(c[0], s, linkTag);
}

export function verify(message: Buffer, anonymitySet: Point[], signatureBuffer: Buffer, linkScope?: Buffer): boolean {
    let n = anonymitySet.length;
    let L = anonymitySet.slice(0);

    let linkBase, linkTag;
    let sig = RingSig.fromBytes(signatureBuffer, !!linkScope);
    if (anonymitySet.length != sig.s.length) {
        throw new Error("given anonymity set and signature anonymity set not of equal length");
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
    if (!ci.equals(sig.c0)) {
        return false;
    }

    return true;
}

function findSecretIndex(keys: Point[], privateKey: Scalar): number {
    const pubKey = Suite.point().base().mul(privateKey);
    const pi = keys.findIndex(pub => pub.equals(pubKey));
    if (pi < 0){
        throw new Error("didn't find public key in anonymity set");
    }

    return pi;
}

function createStreamFromBlake(blakeInstance: BLAKE2Xs): (l: number) => Buffer {
    function getNextBytes(count: number): Buffer {
        let array = new Uint8Array(count);
        blakeInstance.stream(array);

        return Buffer.from(array);
    }

    return getNextBytes;
}

function signH1pre(linkScope: Buffer, linkTag: Point, message: Buffer): BLAKE2Xs {
    const H1pre = new BLAKE2Xs(undefined, {key: message});

    if (linkScope) {
        H1pre.update(linkScope);
        const tag = linkTag.marshalBinary();
        H1pre.update(tag);
    }

    return H1pre;
}

function signH1(H1pre: BLAKE2Xs, PG: Point, PH: Point): Scalar {
    const H1 = cloneDeep(H1pre);
    const PGb = PG.marshalBinary();

    H1.update(PGb);
    if (PH) {
        const PHb = PH.marshalBinary();
        H1.update(PHb);
    }

    return Suite.scalar().pick(createStreamFromBlake(H1));
}
