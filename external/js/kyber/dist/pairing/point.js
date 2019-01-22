"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const crypto_1 = require("crypto");
const bn_1 = require("./bn");
/**
 * Implementation of the point interface for G1
 */
class BN256G1Point {
    constructor(k) {
        this.g1 = new bn_1.G1(k);
    }
    /**
     * Get the element of the point
     * @returns the element
     */
    getElement() {
        return this.g1;
    }
    /** @inheritdoc */
    null() {
        this.g1.setInfinity();
        return this;
    }
    /** @inheritdoc */
    base() {
        this.g1.setBase();
        return this;
    }
    /** @inheritdoc */
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        const bytes = callback(this.g1.marshalSize());
        this.g1 = new bn_1.G1(bytes);
        return this;
    }
    /** @inheritdoc */
    set(p) {
        this.g1 = p.g1.clone();
        return this;
    }
    /** @inheritdoc */
    clone() {
        const p = new BN256G1Point();
        p.set(this);
        return p;
    }
    /** @inheritdoc */
    embedLen() {
        throw new Error('Not implemented');
    }
    /** @inheritdoc */
    embed(data, callback) {
        throw new Error('Not implemented');
    }
    /** @inheritdoc */
    data() {
        throw new Error('Not implemented');
    }
    /** @inheritdoc */
    add(a, b) {
        this.g1.add(a.g1, b.g1);
        return this;
    }
    /** @inheritdoc */
    sub(a, b) {
        return this.add(a, b.neg(b));
    }
    /** @inheritdoc */
    neg(p) {
        this.g1.neg(p.g1);
        return this;
    }
    /** @inheritdoc */
    mul(s, p) {
        this.g1.scalarMul(p.g1, s.getValue());
        return this;
    }
    /**
     * Compute the pairing between the current point and
     * and the provided G2 point
     * @returns the resulting GT point
     */
    pair(g2) {
        return bn_1.GT.pair(this.g1, g2.getElement());
    }
    /** @inheritdoc */
    marshalBinary() {
        return this.g1.marshal();
    }
    /** @inheritdoc */
    unmarshalBinary(buf) {
        this.g1.unmarshal(buf);
    }
    /** @inheritdoc */
    marshalSize() {
        return this.g1.marshalSize();
    }
    /** @inheritdoc */
    equal(p2) {
        return this.g1.equals(p2.g1);
    }
    /** @inheritdoc */
    string() {
        return this.g1.toString();
    }
}
exports.BN256G1Point = BN256G1Point;
/**
 * Implementation of the point interface for G2
 */
class BN256G2Point {
    constructor(k) {
        this.g2 = new bn_1.G2(k);
    }
    /**
     * Get the element of the point
     * @returns the element
     */
    getElement() {
        return this.g2;
    }
    /** @inheritdoc */
    null() {
        this.g2.setInfinity();
        return this;
    }
    /** @inheritdoc */
    base() {
        this.g2.setBase();
        return this;
    }
    /** @inheritdoc */
    pick(callback) {
        callback = callback || crypto_1.randomBytes;
        const bytes = callback(32);
        this.g2 = new bn_1.G2(bytes);
        return this;
    }
    /** @inheritdoc */
    set(p) {
        this.g2 = p.g2.clone();
        return this;
    }
    /** @inheritdoc */
    clone() {
        const p = new BN256G2Point();
        p.set(this);
        return p;
    }
    /** @inheritdoc */
    embedLen() {
        throw new Error("Method not implemented.");
    }
    /** @inheritdoc */
    embed(data, callback) {
        throw new Error("Method not implemented.");
    }
    /** @inheritdoc */
    data() {
        throw new Error("Method not implemented.");
    }
    /** @inheritdoc */
    add(p1, p2) {
        this.g2.add(p1.g2, p2.g2);
        return this;
    }
    /** @inheritdoc */
    sub(p1, p2) {
        return this.add(p1, p2.clone().neg(p2));
    }
    /** @inheritdoc */
    neg(p) {
        this.g2.neg(p.g2);
        return this;
    }
    /** @inheritdoc */
    mul(s, p) {
        this.g2.scalarMul(p.g2, s.getValue());
        return this;
    }
    /**
     * Compute the pairing between the current point and
     * the provided G1 point
     * @returns the resulting GT point
     */
    pair(g1) {
        return bn_1.GT.pair(g1.getElement(), this.g2);
    }
    /** @inheritdoc */
    marshalBinary() {
        return this.g2.marshal();
    }
    /** @inheritdoc */
    unmarshalBinary(bytes) {
        this.g2.unmarshal(bytes);
    }
    /** @inheritdoc */
    marshalSize() {
        return this.g2.marshalSize();
    }
    /** @inheritdoc */
    equal(p2) {
        return this.g2.equals(p2.g2);
    }
    /** @inheritdoc */
    string() {
        return this.g2.toString();
    }
}
exports.BN256G2Point = BN256G2Point;
