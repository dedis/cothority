// types declaration for version 6.4.1 for Kyber usage
//
// The definition contains only a subset of the library

declare module 'elliptic' {
    import BN, { ReductionContext, BNType } from 'bn.js';

    export namespace curve {
        abstract class BaseCurve {
            type: string;
            p: BN;
            red: ReductionContext;
            zero: BN;
            one: BN;
            two: BN;
            n: BN;
            g: BasePoint;
            redN: BN;

            validate(point: BasePoint): boolean;
        }

        class BasePoint {
            curve: BaseCurve;
            type: string;
            x: BN;
            y: BN;

            getX(): BN;
            getY(): BN;
            add(p: BasePoint): BasePoint;
            mul(k: BNType): BasePoint;
        }

        class ShortCurveConf {
            a: string | number | BN | Buffer | number[];
            b: string | number | BN | Buffer | number[];
        }

        export class short extends BaseCurve {
            a: BN;
            b: BN;

            constructor(conf: ShortCurveConf)

            point(x: BNType, y: BNType, isRed?: boolean): BasePoint;
        }

        class EdwardsCurveConf {}

        export class edwards extends BaseCurve {
            point(x: BNType, y: BNType, z?: BNType, t?: BNType): BasePoint;
            pointFromY(y: BNType, odd: boolean): BasePoint;
        }
    }

    export class eddsa {
        curve: curve.edwards;

        constructor(name: string);
    }
}
