// Type definitions for bn.js 4.11.8 for Kyber usage
//
// Uses https://github.com/DefinitelyTyped/DefinitelyTyped/blob/master/types/bn.js/index.d.ts
// as a template but fixes the reduction usage

declare module 'bn.js' {
    type Endianness = 'le' | 'be';
    type IPrimeName = 'k256' | 'p224' | 'p192' | 'p25519';
    export type BNType = number | string | number[] | Buffer | BN;
    export type ReductionContext = {};

    export default class BN {
        constructor(number: BNType, base?: number | 'hex', endian?: Endianness);

        /**
         * create a reduction context
         */
        static red(ctx: BN | IPrimeName): ReductionContext;

        /**
         * create a reduction context  with the Montgomery trick.
         */
        static mont(num: BN): ReductionContext;

        /**
         * returns true if the supplied object is a BN.js instance
         */
        static isBN(b: any): boolean;

        /**
         * returns the maximum of 2 BN instances.
         */
        static max(left: BN, right: BN): BN;

        /**
         * returns the minimum of 2 BN instances.
         */
        static min(left: BN, right: BN): BN;

        /**
         *  clone number
         */
        clone(): BN;

        /**
         *  convert to base-string and pad with zeroes
         */
        toString(base?: number | 'hex', length?: number): string;

        /**
         * convert to Javascript Number (limited to 53 bits)
         */
        toNumber(): number;

        /**
         * convert to JSON compatible hex string (alias of toString(16))
         */
        toJSON(): string;

        /**
         *  convert to byte Array, and optionally zero pad to length, throwing if already exceeding
         */
        toArray(endian?: Endianness, length?: number): number[];

        /**
         * convert to an instance of `type`, which must behave like an Array
         */
        toArrayLike(
            ArrayType: typeof Buffer,
            endian?: Endianness,
            length?: number
        ): Buffer;

        toArrayLike(
            ArrayType: any[],
            endian?: Endianness,
            length?: number
        ): any[];

        /**
         *  convert to Node.js Buffer (if available). For compatibility with browserify and similar tools, use this instead: a.toArrayLike(Buffer, endian, length)
         */
        toBuffer(endian?: Endianness, length?: number): Buffer;

        /**
         * get number of bits occupied
         */
        bitLength(): number;

        /**
         * return number of less-significant consequent zero bits (example: 1010000 has 4 zero bits)
         */
        zeroBits(): number;

        /**
         * return number of bytes occupied
         */
        byteLength(): number;

        /**
         *  true if the number is negative
         */
        isNeg(): boolean;

        /**
         *  check if value is even
         */
        isEven(): boolean;

        /**
         *   check if value is odd
         */
        isOdd(): boolean;

        /**
         *  check if value is zero
         */
        isZero(): boolean;

        /**
         * compare numbers and return `-1 (a < b)`, `0 (a == b)`, or `1 (a > b)` depending on the comparison result
         */
        cmp(b: BN): -1 | 0 | 1;

        /**
         * compare numbers and return `-1 (a < b)`, `0 (a == b)`, or `1 (a > b)` depending on the comparison result
         */
        ucmp(b: BN): -1 | 0 | 1;

        /**
         * compare numbers and return `-1 (a < b)`, `0 (a == b)`, or `1 (a > b)` depending on the comparison result
         */
        cmpn(b: number): -1 | 0 | 1;

        /**
         * a less than b
         */
        lt(b: BN): boolean;

        /**
         * a less than b
         */
        ltn(b: number): boolean;

        /**
         * a less than or equals b
         */
        lte(b: BN): boolean;

        /**
         * a less than or equals b
         */
        lten(b: number): boolean;

        /**
         * a greater than b
         */
        gt(b: BN): boolean;

        /**
         * a greater than b
         */
        gtn(b: number): boolean;

        /**
         * a greater than or equals b
         */
        gte(b: BN): boolean;

        /**
         * a greater than or equals b
         */
        gten(b: number): boolean;

        /**
         * a equals b
         */
        eq(b: BN): boolean;

        /**
         * a equals b
         */
        eqn(b: number): boolean;

        /**
         * convert to two's complement representation, where width is bit width
         */
        toTwos(width: number): BN;

        /**
         * convert from two's complement representation, where width is the bit width
         */
        fromTwos(width: number): BN;

        /**
         * negate sign
         */
        neg(): BN;

        /**
         * negate sign
         */
        ineg(): BN;

        /**
         * absolute value
         */
        abs(): BN;

        /**
         * absolute value
         */
        iabs(): BN;

        /**
         * addition
         */
        add(b: BN): BN;

        /**
         *  addition
         */
        iadd(b: BN): BN;

        /**
         * addition
         */
        addn(b: number): BN;

        /**
         * addition
         */
        iaddn(b: number): BN;

        /**
         * subtraction
         */
        sub(b: BN): BN;

        /**
         * subtraction
         */
        isub(b: BN): BN;

        /**
         * subtraction
         */
        subn(b: number): BN;

        /**
         * subtraction
         */
        isubn(b: number): BN;

        /**
         * multiply
         */
        mul(b: BN): BN;

        /**
         * multiply
         */
        imul(b: BN): BN;

        /**
         * multiply
         */
        muln(b: number): BN;

        /**
         * multiply
         */
        imuln(b: number): BN;

        /**
         * square
         */
        sqr(): BN;

        /**
         * square
         */
        isqr(): BN;

        /**
         * raise `a` to the power of `b`
         */
        pow(b: BN): BN;

        /**
         * divide
         */
        div(b: BN): BN;

        /**
         * divide
         */
        divn(b: number): BN;

        /**
         * divide
         */
        idivn(b: number): BN;

        /**
         * reduct
         */
        mod(b: BN): BN;

        /**
         * reduct
         */
        umod(b: BN): BN;

        /**
         * @see API consistency https://github.com/indutny/bn.js/pull/130
         * reduct
         */
        modn(b: number): number;

        /**
         *  rounded division
         */
        divRound(b: BN): BN;

        /**
         * or
         */
        or(b: BN): BN;

        /**
         * or
         */
        ior(b: BN): BN;

        /**
         * or
         */
        uor(b: BN): BN;

        /**
         * or
         */
        iuor(b: BN): BN;

        /**
         * and
         */
        and(b: BN): BN;

        /**
         * and
         */
        iand(b: BN): BN;

        /**
         * and
         */
        uand(b: BN): BN;

        /**
         * and
         */
        iuand(b: BN): BN;

        /**
         * and (NOTE: `andln` is going to be replaced with `andn` in future)
         */
        andln(b: number): BN;

        /**
         * xor
         */
        xor(b: BN): BN;

        /**
         * xor
         */
        ixor(b: BN): BN;

        /**
         * xor
         */
        uxor(b: BN): BN;

        /**
         * xor
         */
        iuxor(b: BN): BN;

        /**
         * set specified bit to 1
         */
        setn(b: number): BN;

        /**
         * shift left
         */
        shln(b: number): BN;

        /**
         * shift left
         */
        ishln(b: number): BN;

        /**
         * shift left
         */
        ushln(b: number): BN;

        /**
         * shift left
         */
        iushln(b: number): BN;

        /**
         * shift right
         */
        shrn(b: number): BN;

        /**
         * shift right (unimplemented https://github.com/indutny/bn.js/blob/master/lib/bn.js#L2086)
         */
        ishrn(b: number): BN;

        /**
         * shift right
         */
        ushrn(b: number): BN;

        /**
         * shift right
         */
        iushrn(b: number): BN;

        /**
         *  test if specified bit is set
         */
        testn(b: number): boolean;

        /**
         * clear bits with indexes higher or equal to `b`
         */
        maskn(b: number): BN;

        /**
         * clear bits with indexes higher or equal to `b`
         */
        imaskn(b: number): BN;

        /**
         * add `1 << b` to the number
         */
        bincn(b: number): BN;

        /**
         * not (for the width specified by `w`)
         */
        notn(w: number): BN;

        /**
         * not (for the width specified by `w`)
         */
        inotn(w: number): BN;

        /**
         * GCD
         */
        gcd(b: BN): BN;

        /**
         * Extended GCD results `({ a: ..., b: ..., gcd: ... })`
         */
        egcd(b: BN): { a: BN; b: BN; gcd: BN };

        /**
         * inverse `a` modulo `b`
         */
        invm(b: BN): BN;

        /**
         *  Convert number to red
         */
        toRed(ctx: ReductionContext): BN;

        /**
         * Convert back a number using a reduction context
         */
        fromRed(): BN;

        forceRed(ctx: ReductionContext): BN;

        /**
         * modular addition
         */
        redAdd(b: BN): BN;

        /**
         * in-place modular addition
         */
        redIAdd(b: BN): BN;

        /**
         * modular subtraction
         */
        redSub(b: BN): BN;

        /**
         * in-place modular subtraction
         */
        redISub(b: BN): BN;

        /**
         * modular shift left
         */
        redShl(num: number): BN;

        /**
         * modular multiplication
         */
        redMul(b: BN): BN;

        /**
         * in-place modular multiplication
         */
        redIMul(b: BN): BN;

        /**
         * modular square
         */
        redSqr(): BN;

        /**
         * in-place modular square root
         */
        redISqr(): BN;

        /**
         * square root modulo reduction context's prime
         */
        redSqrt(): BN;

        /**
         * modular inverse of the number
         */
        redInvm(): BN;

        /**
         * modular negation
         */
        redNeg(): BN;

        /**
         * modular exponentiation
         */
        redPow(b: BN): BN;
    }
}
