// tslint:disable:variable-name
import { BLAKE2Xs } from "@stablelib/blake2xs";
import { cloneDeep } from "lodash";
import * as curve from "../../curve";
import { Point, Scalar } from "../../suite";

// tslint:disable-next-line
export const Suite = curve.newCurve("edwards25519");

export class RingSig {

    static fromBytes(signatureBuffer: Buffer, isLinkableSig: boolean): RingSig {
        const scalarMarshalSize = Suite.scalarLen();
        const pointMarshalSize = Suite.pointLen();

        const c0 = Suite.scalar();
        c0.unmarshalBinary(signatureBuffer.slice(0, pointMarshalSize));

        const S: Scalar[] = [];
        const endIndex = isLinkableSig ? signatureBuffer.length - pointMarshalSize : signatureBuffer.length;
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
    readonly c0: Scalar;
    readonly s: Scalar[];
    readonly tag: Point;

    constructor(c0: Scalar, s: Scalar[], tag?: Point) {
        this.c0 = c0;
        this.s = s;
        this.tag = tag;
    }

    encode(): Buffer {
        const bufs: Buffer[] = [];

        bufs.push(this.c0.marshalBinary());

        for (const scalar of this.s) {
            bufs.push(scalar.marshalBinary());
        }

        if (this.tag) {
            bufs.push(this.tag.marshalBinary());
        }

        return Buffer.concat(bufs);
    }
}

export function sign(message: Buffer, anonymitySet: Point[], secret: Scalar, linkScope?: Buffer) {
    const hasLS = linkScope && (linkScope !== null);

    const pi = findSecretIndex(anonymitySet, secret);
    const n = anonymitySet.length;
    const L = anonymitySet.slice(0);

    let linkBase;
    let linkTag: Point;
    if (hasLS) {
        const linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = Suite.point().mul(secret, linkBase);
    }

    const H1pre = signH1pre(linkScope, linkTag, message);

    const u = Suite.scalar().pick();
    const UB = Suite.point().mul(u);
    let UL;
    if (hasLS) {
        UL = Suite.point().mul(u, linkBase);
    }

    const s: Scalar[] = [];
    const c: Scalar[] = [];

    c[(pi + 1) % n] = signH1(H1pre, UB, UL);

    const P = Suite.point();
    const PG = Suite.point();
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
    const n = anonymitySet.length;
    const L = anonymitySet.slice(0);

    let linkBase;
    let linkTag;
    const sig = RingSig.fromBytes(signatureBuffer, !!linkScope);
    if (anonymitySet.length !== sig.s.length) {
        throw new Error("given anonymity set and signature anonymity set not of equal length");
    }

    if (linkScope) {
        const linkStream = new BLAKE2Xs(undefined, {key: linkScope});
        linkBase = Suite.point().pick(createStreamFromBlake(linkStream));
        linkTag = sig.tag;
    }

    const H1pre = signH1pre(linkScope, linkTag, message);

    let PH: Point;
    const P = Suite.point();
    const PG = Suite.point();
    if (linkScope) {
        PH = Suite.point();
    }
    const s = sig.s;
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
    const pi = keys.findIndex((pub) => pub.equals(pubKey));
    if (pi < 0) {
        throw new Error("didn't find public key in anonymity set");
    }

    return pi;
}

function createStreamFromBlake(blakeInstance: BLAKE2Xs): (l: number) => Buffer {
    function getNextBytes(count: number): Buffer {
        const array = new Uint8Array(count);
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
