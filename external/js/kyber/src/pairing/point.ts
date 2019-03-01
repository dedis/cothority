import { randomBytes } from 'crypto';
import { Point } from '../index';
import { G1, G2, GT, BNType } from './bn';
import BN256Scalar from './scalar';

/**
 * Implementation of the point interface for G1
 */
export class BN256G1Point implements Point {
    public static MARSHAL_ID = Buffer.from('bn256.g1');

    /**
     * Hash the message into a point
     * @param msg The message to hash
     * @returns a valid point
     */
    public static hashToPoint(msg: Buffer): BN256G1Point {
        const p = new BN256G1Point();
        p.g1 = G1.hashToPoint(msg);

        return p;
    }

    private g1: G1;

    constructor(k?: BNType) {
        this.g1 = new G1(k);
    }

    /**
     * Get the element of the point
     * @returns the element
     */
    getElement(): G1 {
        return this.g1;
    }

    /** @inheritdoc */
    null(): BN256G1Point {
        this.g1.setInfinity();
        return this;
    }

    /** @inheritdoc */
    base(): BN256G1Point {
        this.g1.setBase();
        return this;
    }

    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256G1Point {
        callback = callback || randomBytes;
        const bytes = callback(this.g1.marshalSize());
        this.g1 = new G1(bytes);

        return this;
    }

    /** @inheritdoc */
    set(p: BN256G1Point): BN256G1Point {
        this.g1 = p.g1.clone();
        return this;
    }

    /** @inheritdoc */
    clone(): BN256G1Point {
        const p = new BN256G1Point();
        p.set(this);
        return p;
    }

    /** @inheritdoc */
    embedLen(): number {
        throw new Error('Not implemented');
    }

    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): BN256G1Point {
        throw new Error('Not implemented');
    }

    /** @inheritdoc */
    data(): Buffer {
        throw new Error('Not implemented');
    }

    /** @inheritdoc */
    add(a: BN256G1Point, b: BN256G1Point): BN256G1Point {
        this.g1.add(a.g1, b.g1);
        return this;
    }

    /** @inheritdoc */
    sub(a: BN256G1Point, b: BN256G1Point): BN256G1Point {
        return this.add(a, b.neg(b));
    }

    /** @inheritdoc */
    neg(p: BN256G1Point): BN256G1Point {
        this.g1.neg(p.g1);
        return this;
    }

    /** @inheritdoc */
    mul(s: BN256Scalar, p: BN256G1Point): BN256G1Point {
        this.g1.scalarMul(p.g1, s.getValue());
        return this;
    }

    /**
     * Compute the pairing between the current point and
     * and the provided G2 point
     * @returns the resulting GT point
     */
    pair(g2: BN256G2Point): GT {
        return GT.pair(this.g1, g2.getElement());
    }

    /** @inheritdoc */
    marshalBinary(): Buffer {
        return this.g1.marshal();
    }

    /** @inheritdoc */
    unmarshalBinary(buf: Buffer): void {
        this.g1.unmarshal(buf);
    }

    /** @inheritdoc */
    marshalSize(): number {
        return this.g1.marshalSize();
    }

    /** @inheritdoc */
    equals(p2: Point): p2 is BN256G1Point {
        if (!(p2 instanceof BN256G1Point)) {
            return false;
        }

        return this.g1.equals(p2.g1);
    }

    /** @inheritdoc */
    toString(): string {
        return this.g1.toString();
    }

    /** @inheritdoc */
    toProto(): Buffer {
        return Buffer.concat([BN256G1Point.MARSHAL_ID, this.marshalBinary()]);
    }
}

/**
 * Implementation of the point interface for G2
 */
export class BN256G2Point implements Point {
    public static MARSHAL_ID = Buffer.from('bn256.g2');

    private g2: G2;

    constructor(k?: BNType) {
        this.g2 = new G2(k);
    }

    /**
     * Get the element of the point
     * @returns the element
     */
    getElement(): G2 {
        return this.g2;
    }

    /** @inheritdoc */
    null(): BN256G2Point {
        this.g2.setInfinity();
        return this;
    }

    /** @inheritdoc */
    base(): BN256G2Point {
        this.g2.setBase();
        return this;
    }

    /** @inheritdoc */
    pick(callback?: (length: number) => Buffer): BN256G2Point {
        callback = callback || randomBytes;
        const bytes = callback(32);
        this.g2 = new G2(bytes);

        return this;
    }

    /** @inheritdoc */
    set(p: BN256G2Point): BN256G2Point {
        this.g2 = p.g2.clone();
        return this;
    }

    /** @inheritdoc */
    clone(): BN256G2Point {
        const p = new BN256G2Point();
        p.set(this);
        return p;
    }

    /** @inheritdoc */
    embedLen(): number {
        throw new Error("Method not implemented.");
    }

    /** @inheritdoc */
    embed(data: Buffer, callback?: (length: number) => Buffer): BN256G2Point {
        throw new Error("Method not implemented.");
    }

    /** @inheritdoc */
    data(): Buffer {
        throw new Error("Method not implemented.");
    }

    /** @inheritdoc */
    add(p1: BN256G2Point, p2: BN256G2Point): BN256G2Point {
        this.g2.add(p1.g2, p2.g2);
        return this;
    }

    /** @inheritdoc */
    sub(p1: BN256G2Point, p2: BN256G2Point): BN256G2Point {
        return this.add(p1, p2.clone().neg(p2));
    }

    /** @inheritdoc */
    neg(p: BN256G2Point): BN256G2Point {
        this.g2.neg(p.g2);
        return this;
    }

    /** @inheritdoc */
    mul(s: BN256Scalar, p?: BN256G2Point): BN256G2Point {
        this.g2.scalarMul(p.g2, s.getValue());
        return this;
    }

    /**
     * Compute the pairing between the current point and
     * the provided G1 point
     * @returns the resulting GT point
     */
    pair(g1: BN256G1Point): GT {
        return GT.pair(g1.getElement(), this.g2);
    }

    /** @inheritdoc */
    marshalBinary(): Buffer {
        return this.g2.marshal();
    }

    /** @inheritdoc */
    unmarshalBinary(bytes: Buffer): void {
        this.g2.unmarshal(bytes);
    }

    /** @inheritdoc */
    marshalSize(): number {
        return this.g2.marshalSize();
    }

    /** @inheritdoc */
    equals(p2: Point): p2 is BN256G2Point {
        if (!(p2 instanceof BN256G2Point)) {
            return false;
        }

        return this.g2.equals(p2.g2);
    }

    /** @inheritdoc */
    toString(): string {
        return this.g2.toString();
    }

    /** @inheritdoc */
    toProto(): Buffer {
        return Buffer.concat([BN256G2Point.MARSHAL_ID, this.marshalBinary()]);
    }
}
