import BN = require("bn.js");
import NistPoint from "./point";
import NistScalar from "./scalar";
import Weierstrass from "./curve";
declare const _default: {
    Point: typeof NistPoint;
    Scalar: typeof NistScalar;
    Curve: typeof Weierstrass;
    Params: {
        p256: {
            name: string;
            bitSize: number;
            p: BN;
            a: BN;
            b: BN;
            n: BN;
            gx: BN;
            gy: BN;
        };
    };
};
export default _default;
