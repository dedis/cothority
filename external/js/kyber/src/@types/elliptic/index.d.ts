// types declaration for version 6.4.1 for Kyber usage
//
// The definition contains only a subset of the library

declare module 'elliptic' {
    import BN = require('bn.js');

    type BNType = number | string | number[] | Buffer | BN;

    interface ReductionContext {
        prime: any;
        m: any;
    }

    export namespace curve {
        class base {
            constructor(type: string, conf: base.BaseCurveOptions)
            p: BN;
            type: string;
            red: ReductionContext;
            zero: BN;
            one: BN;
            two: BN;
            n: BN;
            g: base.BasePoint;
            redN: BN;

            validate(point: base.BasePoint): boolean;
            decodePoint(bytes: Buffer | string, enc?: 'hex'): base.BasePoint;
        }
    
        namespace base {
            class BasePoint {
                curve: base;
                type: string;
                precomputed: PrecomputedValues | null;

                constructor(curve: base, type: string);
                
                encode(enc: string, compact: boolean): string | Buffer;
                encodeCompressed(enc: string): BN;
                validate(): boolean;
                precompute(power: number): BasePoint;
                dblp(k: number): BasePoint;
                inspect(): string;
                isInfinity(): boolean;
                add(p: BasePoint): BasePoint;
                mul(k: BNType): BasePoint;
                dbl(): BasePoint;
                getX(): BN;
                getY(): BN;
                eq(p: BasePoint): boolean;
                neg(): BasePoint;
            }
    
            interface BaseCurveOptions {
                p: number | string | number[] | Buffer | BN;
                prime?: BN | string;
                n?: number | BN | Buffer;
                g?: BasePoint;
                gRed?: any; // ?
            }
    
            interface PrecomputedValues {
                doubles: any; // ?
                naf: any; // ?
                beta: any; // ?
            }
        }

        class edwards extends base {
            constructor(conf: edwards.EdwardsConf);

            point(x: BNType, y: BNType, z?: BNType, t?: BNType): edwards.EdwardsPoint;
            pointFromX(x: BNType, odd?: boolean): edwards.EdwardsPoint
            pointFromY(y: BNType, odd?: boolean): edwards.EdwardsPoint;
            pointFromJSON(obj: BNType[]): edwards.EdwardsPoint;
        }

        namespace edwards {
            interface EdwardsConf extends base.BaseCurveOptions {
                a: BNType;
                c: BNType;
                d: BNType;
            }

            class EdwardsPoint extends base.BasePoint {
                normalize(): EdwardsPoint;
                eqXToP(x: BN): boolean;
            }
        }

        class short extends base {
            a: BN;
            b: BN;
            g: short.ShortPoint;

            constructor(conf: short.ShortConf);

            point(x: BNType, y: BNType, isRed?: boolean): short.ShortPoint;
            pointFromX(x: BNType, odd?: boolean): short.ShortPoint;
            pointFromJSON(obj: BNType[], red: boolean): short.ShortPoint;
        }

        namespace short {
            interface ShortConf extends base.BaseCurveOptions {
                a: BNType,
                b: BNType,
                beta?: BNType,
                lambda?: BNType,
            }

            class ShortPoint extends base.BasePoint {
                x: BN;
                y: BN;

                toJSON(): BNType[];
            }
        }
    }

    export class eddsa {
        curve: curve.edwards;
    
        constructor(name: 'ed25519');
    
        sign(message: eddsa.Bytes, secret: eddsa.Bytes): eddsa.Signature;
        verify(message: eddsa.Bytes, sig: eddsa.Bytes | eddsa.Signature, pub: eddsa.Bytes | eddsa.Point | eddsa.KeyPair): boolean;
        hashInt(): BN;
        keyFromPublic(pub: eddsa.Bytes): eddsa.KeyPair;
        keyFromSecret(secret: eddsa.Bytes): eddsa.KeyPair;
        makeSignature(sig: eddsa.Signature | Buffer | string): eddsa.Signature;
        encodePoint(point: eddsa.Point): Buffer;
        decodePoint(bytes: eddsa.Bytes): eddsa.Point;
        encodeInt(num: BN): Buffer;
        decodeInt(bytes: BNType): BN;
        isPoint(val: any): boolean;
    }
    
    export namespace eddsa {
        type Point = curve.base.BasePoint;
        type Bytes = string | Buffer;
    
        class Signature {
            constructor(eddsa: eddsa, sig: Signature | Bytes);
    
            toBytes(): Buffer;
            toHex(): string;
        }
    
        class KeyPair {
            constructor(eddsa: eddsa, params: KeyPairOptions);
    
            static fromPublic(eddsa: eddsa, pub: Bytes): KeyPair;
            static fromSecret(eddsa: eddsa, secret: Bytes): KeyPair;
    
            secret(): Buffer;
            sign(message: Bytes): Signature;
            verify(message: Bytes, sig: Signature | Bytes): boolean;
            getSecret(enc: 'hex'): string;
            getSecret(): Buffer;
            getPublic(enc: 'hex'): string;
            getPublic(): Buffer;
        }
    
        interface KeyPairOptions {
            secret: Buffer;
            pub: Buffer | Point;
        }
    }
}
