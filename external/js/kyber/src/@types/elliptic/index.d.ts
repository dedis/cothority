// types declaration for version 6.4.1 for Kyber usage
//
// The definition contains only a subset of the library

declare module 'elliptic' {
    import BN, { ReductionContext, BNType } from 'bn.js';

    export namespace curve {
        class BaseCurve {
            type: string;
            p: BN;
            red: ReductionContext;
            zero: BN;
            one: BN;
            two: BN;
            n: BN;
            g: BasePoint;
            redN: BN;

            point(x: BNType, y: BNType, isRed?: boolean): BasePoint;
            validate(point: BasePoint): boolean;
        }

        class BasePoint {
            curve: BaseCurve;
            type: string;
            x: BN;
            y: BN;

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
        }
    }

    export class eddsa {
        curve: curve.BaseCurve;

        constructor(name: string);
    }
}
